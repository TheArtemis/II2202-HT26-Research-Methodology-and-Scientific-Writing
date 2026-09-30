#!/usr/bin/env python3
"""Reproduce the exoplanet analysis and every table/figure in the report.

The script is deliberately offline: it reads the dated NASA Exoplanet Archive
snapshot in data/exoplanets.csv and writes outputs to results/ and figures/.
"""

from __future__ import annotations

import json
import platform
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import sklearn
from sklearn.base import clone
from sklearn.dummy import DummyRegressor
from sklearn.ensemble import RandomForestRegressor
from sklearn.inspection import permutation_importance
from sklearn.linear_model import LinearRegression
from sklearn.metrics import mean_absolute_error, mean_squared_error, r2_score
from sklearn.model_selection import GroupKFold


SEED = 2202
ROOT = Path(__file__).resolve().parent
DATA_PATH = ROOT / "data" / "exoplanets.csv"
RESULTS_DIR = ROOT / "results"
FIGURES_DIR = ROOT / "figures"

EXPECTED_COLUMNS = [
    "pl_name",
    "hostname",
    "disc_year",
    "discoverymethod",
    "pl_rade",
    "pl_bmasse",
    "pl_bmassprov",
    "pl_orbper",
    "pl_eqt",
    "pl_insol",
    "pl_orbsmax",
    "st_teff",
    "st_rad",
    "st_mass",
    "st_met",
    "st_age",
    "sy_dist",
]

REQUIRED_MODEL_COLUMNS = [
    "pl_rade",
    "pl_bmasse",
    "pl_orbper",
    "pl_eqt",
    "st_mass",
    "st_met",
]

RAW_LABELS = {
    "pl_rade": "Planet radius",
    "pl_bmasse": "Planet mass",
    "pl_orbper": "Orbital period",
    "pl_eqt": "Equilibrium temperature",
    "st_mass": "Stellar mass",
    "st_met": "Stellar metallicity",
}

FEATURE_LABELS = {
    "log_planet_mass": "Planet mass",
    "log_orbital_period": "Orbital period",
    "log_equilibrium_temperature": "Equilibrium temperature",
    "stellar_mass": "Stellar mass",
    "stellar_metallicity": "Stellar metallicity",
}


def load_and_select() -> tuple[pd.DataFrame, dict]:
    """Validate the archive snapshot and create the pre-specified model sample."""
    raw = pd.read_csv(DATA_PATH)
    if list(raw.columns) != EXPECTED_COLUMNS:
        raise ValueError("Unexpected NASA archive schema; check README query and snapshot")
    if raw["pl_name"].isna().any() or raw["hostname"].isna().any():
        raise ValueError("Planet or host identifiers are missing")

    transit = raw.loc[raw["discoverymethod"].eq("Transit")].copy()
    measured_mass = transit.loc[transit["pl_bmassprov"].eq("Mass")].copy()
    complete = measured_mass.dropna(subset=REQUIRED_MODEL_COLUMNS).copy()

    positive_columns = ["pl_rade", "pl_bmasse", "pl_orbper", "pl_eqt", "st_mass"]
    nonpositive_mask = complete[positive_columns].le(0).any(axis=1)
    nonpositive_rows = int(nonpositive_mask.sum())
    complete = complete.loc[~nonpositive_mask].copy()
    complete = complete.sort_values("pl_name").reset_index(drop=True)

    if complete["pl_name"].duplicated().any():
        raise ValueError("The composite table unexpectedly contains duplicate planet names")

    selection = pd.DataFrame(
        [
            ("Confirmed planets with reported radius", len(raw), "TAP query condition: pl_rade is not null"),
            ("Transit discoveries", len(transit), "Consistent radius-detection context"),
            (
                "Transit discoveries with measured mass",
                len(measured_mass),
                "pl_bmassprov = Mass; excludes radius-derived M-R relationship values",
            ),
            (
                "Complete, physically valid model sample",
                len(complete),
                "Complete required predictors and positive log-transformed quantities",
            ),
        ],
        columns=["selection_step", "rows_remaining", "criterion"],
    )
    selection.to_csv(RESULTS_DIR / "sample_selection.csv", index=False)

    quality = {
        "archive_rows_with_radius": int(len(raw)),
        "transit_rows": int(len(transit)),
        "transit_rows_with_measured_mass": int(len(measured_mass)),
        "complete_model_rows": int(len(complete)),
        "unique_host_stars_in_model_sample": int(complete["hostname"].nunique()),
        "duplicate_planet_names_in_archive": int(raw["pl_name"].duplicated().sum()),
        "model_rows_excluded_for_missing_values": int(len(measured_mass) - len(complete) - nonpositive_rows),
        "model_rows_excluded_for_nonpositive_values": nonpositive_rows,
        "missing_by_required_column_before_complete_case_selection": {
            column: int(measured_mass[column].isna().sum()) for column in REQUIRED_MODEL_COLUMNS
        },
        "mass_provenance_counts_among_transit_planets": {
            str(key): int(value)
            for key, value in transit["pl_bmassprov"].value_counts(dropna=False).items()
        },
    }
    return complete, quality


def model_features(data: pd.DataFrame) -> pd.DataFrame:
    """Apply transformations fixed before validation."""
    return pd.DataFrame(
        {
            "log_planet_mass": np.log10(data["pl_bmasse"]),
            "log_orbital_period": np.log10(data["pl_orbper"]),
            "log_equilibrium_temperature": np.log10(data["pl_eqt"]),
            "stellar_mass": data["st_mass"],
            "stellar_metallicity": data["st_met"],
        },
        index=data.index,
    )


def descriptive_analysis(data: pd.DataFrame) -> tuple[pd.DataFrame, pd.Series]:
    selected = data[REQUIRED_MODEL_COLUMNS]
    desc = selected.describe(percentiles=[0.01, 0.25, 0.50, 0.75, 0.99]).T
    desc = desc.rename(
        columns={"std": "sd", "1%": "p01", "25%": "q1", "50%": "median", "75%": "q3", "99%": "p99"}
    )
    desc["skewness"] = selected.skew()
    desc.index = [RAW_LABELS[index] for index in desc.index]
    desc.to_csv(RESULTS_DIR / "descriptive_statistics.csv", float_format="%.5f")

    spearman = selected.corr(method="spearman")["pl_rade"].drop("pl_rade").sort_values()
    spearman.rename(index=RAW_LABELS).to_csv(
        RESULTS_DIR / "spearman_correlations_with_radius.csv",
        header=["spearman_rho"],
        float_format="%.5f",
    )
    return desc, spearman


def regression_metrics(observed_log: np.ndarray, predicted_log: np.ndarray) -> dict[str, float]:
    observed = np.power(10.0, observed_log)
    predicted = np.power(10.0, predicted_log)
    return {
        "rmse_earth_radii": float(mean_squared_error(observed, predicted) ** 0.5),
        "mae_earth_radii": float(mean_absolute_error(observed, predicted)),
        "r_squared": float(r2_score(observed, predicted)),
        "log_rmse_dex": float(mean_squared_error(observed_log, predicted_log) ** 0.5),
    }


def model_analysis(
    data: pd.DataFrame,
) -> tuple[pd.DataFrame, dict, pd.DataFrame, dict[str, np.ndarray]]:
    X = model_features(data)
    y_log = np.log10(data["pl_rade"]).to_numpy()
    groups = data["hostname"].to_numpy()
    models = {
        "Median baseline": DummyRegressor(strategy="median"),
        "Log-linear regression": LinearRegression(),
        "Random forest": RandomForestRegressor(
            n_estimators=400,
            min_samples_leaf=2,
            max_features=1.0,
            random_state=SEED,
            n_jobs=1,
        ),
    }

    splitter = GroupKFold(n_splits=10)
    rows: list[dict] = []
    predictions = {name: np.full(len(data), np.nan) for name in models}
    importance_rows: list[dict] = []

    for fold, (train_index, test_index) in enumerate(splitter.split(X, y_log, groups), start=1):
        X_train, X_test = X.iloc[train_index], X.iloc[test_index]
        y_train, y_test = y_log[train_index], y_log[test_index]
        for name, estimator in models.items():
            fitted = clone(estimator).fit(X_train, y_train)
            predicted = fitted.predict(X_test)
            predictions[name][test_index] = predicted
            metrics = regression_metrics(y_test, predicted)
            rows.append(
                {
                    "fold": fold,
                    "model": name,
                    "n_train": len(train_index),
                    "n_test": len(test_index),
                    "train_hosts": int(data.iloc[train_index]["hostname"].nunique()),
                    "test_hosts": int(data.iloc[test_index]["hostname"].nunique()),
                    **metrics,
                }
            )

            if name == "Random forest":
                result = permutation_importance(
                    fitted,
                    X_test,
                    y_test,
                    scoring="neg_root_mean_squared_error",
                    n_repeats=12,
                    random_state=SEED + fold,
                    n_jobs=1,
                )
                for feature_index, feature in enumerate(X.columns):
                    for repeat, increase in enumerate(result.importances[feature_index], start=1):
                        importance_rows.append(
                            {
                                "fold": fold,
                                "repeat": repeat,
                                "feature": FEATURE_LABELS[feature],
                                "log_rmse_increase_dex": increase,
                            }
                        )

    fold_metrics = pd.DataFrame(rows)
    fold_metrics.to_csv(RESULTS_DIR / "cross_validation_folds.csv", index=False, float_format="%.6f")
    metric_columns = ["rmse_earth_radii", "mae_earth_radii", "r_squared", "log_rmse_dex"]
    summary = fold_metrics.groupby("model")[metric_columns].agg(["mean", "std"]).round(5)
    summary.to_csv(RESULTS_DIR / "cross_validation_summary.csv")

    importance_raw = pd.DataFrame(importance_rows)
    importance = (
        importance_raw.groupby("feature")["log_rmse_increase_dex"]
        .agg(["mean", "std"])
        .sort_values("mean", ascending=False)
    )
    importance.to_csv(RESULTS_DIR / "permutation_importance.csv", float_format="%.6f")

    pooled = {name: regression_metrics(y_log, prediction) for name, prediction in predictions.items()}
    observed = np.power(10.0, y_log)
    linear = np.power(10.0, predictions["Log-linear regression"])
    forest = np.power(10.0, predictions["Random forest"])

    # Resample complete host systems, not individual planets, to preserve clustering.
    host_to_indices = {host: np.flatnonzero(groups == host) for host in np.unique(groups)}
    unique_hosts = np.array(list(host_to_indices))
    rng = np.random.default_rng(SEED)
    differences = np.empty(5000)
    for iteration in range(len(differences)):
        sampled_hosts = rng.choice(unique_hosts, size=len(unique_hosts), replace=True)
        sample = np.concatenate([host_to_indices[host] for host in sampled_hosts])
        linear_rmse = np.sqrt(np.mean((observed[sample] - linear[sample]) ** 2))
        forest_rmse = np.sqrt(np.mean((observed[sample] - forest[sample]) ** 2))
        differences[iteration] = linear_rmse - forest_rmse

    inference = {
        "pooled_out_of_fold_metrics": pooled,
        "pooled_forest_vs_linear_rmse_reduction_earth_radii": float(
            pooled["Log-linear regression"]["rmse_earth_radii"]
            - pooled["Random forest"]["rmse_earth_radii"]
        ),
        "host_cluster_bootstrap_rmse_reduction_95_percent_ci_earth_radii": [
            float(value) for value in np.quantile(differences, [0.025, 0.975])
        ],
        "bootstrap_resamples": int(len(differences)),
    }
    return fold_metrics, inference, importance, predictions


def learning_curve_analysis(data: pd.DataFrame) -> pd.DataFrame:
    """Estimate error by training size while keeping host systems intact."""
    X = model_features(data)
    y_log = np.log10(data["pl_rade"]).to_numpy()
    groups = data["hostname"].to_numpy()
    models = {
        "Log-linear regression": LinearRegression(),
        "Random forest": RandomForestRegressor(
            n_estimators=400,
            min_samples_leaf=2,
            max_features=1.0,
            random_state=SEED,
            n_jobs=1,
        ),
    }
    splitter = GroupKFold(n_splits=5)
    fractions = [0.10, 0.25, 0.50, 0.75, 1.00]
    rows: list[dict] = []

    for fold, (train_index, test_index) in enumerate(splitter.split(X, y_log, groups), start=1):
        rng = np.random.default_rng(SEED + fold)
        train_hosts = np.unique(groups[train_index])
        shuffled_hosts = rng.permutation(train_hosts)
        for fraction in fractions:
            host_count = max(20, int(round(len(shuffled_hosts) * fraction)))
            selected_hosts = shuffled_hosts[:host_count]
            subset = train_index[np.isin(groups[train_index], selected_hosts)]
            for name, estimator in models.items():
                fitted = clone(estimator).fit(X.iloc[subset], y_log[subset])
                predicted = fitted.predict(X.iloc[test_index])
                metrics = regression_metrics(y_log[test_index], predicted)
                rows.append(
                    {
                        "fold": fold,
                        "model": name,
                        "training_fraction": fraction,
                        "training_rows": len(subset),
                        "training_hosts": host_count,
                        "rmse_earth_radii": metrics["rmse_earth_radii"],
                    }
                )

    curve = pd.DataFrame(rows)
    curve.to_csv(RESULTS_DIR / "learning_curve_folds.csv", index=False, float_format="%.6f")
    summary = (
        curve.groupby(["model", "training_fraction"])
        .agg(
            training_rows_mean=("training_rows", "mean"),
            training_hosts_mean=("training_hosts", "mean"),
            rmse_mean=("rmse_earth_radii", "mean"),
            rmse_sd=("rmse_earth_radii", "std"),
        )
        .reset_index()
    )
    summary.to_csv(RESULTS_DIR / "learning_curve_summary.csv", index=False, float_format="%.6f")
    return summary


def configure_plots() -> None:
    plt.rcParams.update(
        {
            "font.size": 9,
            "axes.titlesize": 10,
            "axes.labelsize": 9,
            "legend.fontsize": 8,
            "figure.dpi": 150,
            "savefig.dpi": 300,
            "savefig.bbox": "tight",
            "axes.spines.top": False,
            "axes.spines.right": False,
            "pdf.fonttype": 42,
        }
    )


def plot_exploration(data: pd.DataFrame, spearman: pd.Series) -> None:
    configure_plots()
    fig, axes = plt.subplots(2, 2, figsize=(7.1, 6.1))

    axes[0, 0].hist(data["pl_rade"], bins=28, color="0.72", edgecolor="black", linewidth=0.6)
    median_radius = data["pl_rade"].median()
    axes[0, 0].axvline(
        median_radius,
        color="black",
        linestyle="--",
        linewidth=1.2,
        label=f"Median = {median_radius:.2f} Earth radii",
    )
    axes[0, 0].set(
        xlabel="Planet radius (Earth radii)",
        ylabel="Planets",
        title="(a) Outcome distribution",
    )
    axes[0, 0].legend(frameon=False)

    axes[0, 1].scatter(
        data["pl_bmasse"],
        data["pl_rade"],
        s=11,
        facecolors="none",
        edgecolors="0.45",
        linewidths=0.45,
        alpha=0.55,
        label="Planet",
    )
    mass_bins = pd.qcut(np.log10(data["pl_bmasse"]), q=12, duplicates="drop")
    mass_summary = data.groupby(mass_bins, observed=True)[["pl_bmasse", "pl_rade"]].median()
    axes[0, 1].plot(
        mass_summary["pl_bmasse"],
        mass_summary["pl_rade"],
        color="black",
        marker="o",
        markersize=3,
        linewidth=1.2,
        label="Median by mass bin",
    )
    axes[0, 1].set_xscale("log")
    axes[0, 1].set_yscale("log")
    axes[0, 1].set(
        xlabel="Planet mass (Earth masses, log scale)",
        ylabel="Planet radius (Earth radii, log scale)",
        title="(b) Nonlinear mass--radius relationship",
    )
    axes[0, 1].legend(frameon=False)

    axes[1, 0].scatter(
        data["pl_eqt"],
        data["pl_rade"],
        s=11,
        facecolors="none",
        edgecolors="0.40",
        linewidths=0.45,
        alpha=0.55,
    )
    axes[1, 0].set(
        xlabel="Equilibrium temperature (K)",
        ylabel="Planet radius (Earth radii)",
        title="(c) Radius and equilibrium temperature",
    )

    ordered = spearman.sort_values()
    bars = axes[1, 1].barh(
        [RAW_LABELS[index] for index in ordered.index],
        ordered.values,
        color=["0.80" if value < 0 else "0.35" for value in ordered.values],
        edgecolor="black",
        linewidth=0.5,
    )
    axes[1, 1].axvline(0, color="black", linewidth=0.7)
    for bar, value in zip(bars, ordered.values):
        label_x = value + 0.018
        axes[1, 1].text(
            label_x,
            bar.get_y() + bar.get_height() / 2,
            f"{value:.2f}",
            va="center",
            ha="left",
            fontsize=7,
        )
    axes[1, 1].set_xlim(-0.36, 0.90)
    axes[1, 1].set(
        xlabel="Spearman correlation with radius",
        title="(d) Unadjusted monotonic associations",
    )

    fig.tight_layout(pad=1.2)
    fig.savefig(FIGURES_DIR / "exploratory_analysis.pdf")
    fig.savefig(FIGURES_DIR / "exploratory_analysis.png")
    plt.close(fig)


def plot_predictions(data: pd.DataFrame, predictions: dict[str, np.ndarray], inference: dict) -> None:
    configure_plots()
    displayed = ["Log-linear regression", "Random forest"]
    markers = ["o", "x"]
    fig, axes = plt.subplots(1, 2, figsize=(7.1, 3.25), sharex=True, sharey=True)
    observed = data["pl_rade"].to_numpy()
    limits = [0.25, 30]
    for axis, name, marker in zip(axes, displayed, markers):
        predicted = np.power(10.0, predictions[name])
        axis.scatter(observed, predicted, s=13, marker=marker, color="0.25", alpha=0.48, linewidths=0.6)
        axis.plot(limits, limits, color="black", linestyle="--", linewidth=1, label="Perfect prediction")
        metrics = inference["pooled_out_of_fold_metrics"][name]
        axis.text(
            0.04,
            0.94,
            f"RMSE = {metrics['rmse_earth_radii']:.2f} Earth radii\n$R^2$ = {metrics['r_squared']:.3f}",
            transform=axis.transAxes,
            va="top",
            bbox={"boxstyle": "round,pad=0.25", "facecolor": "white", "edgecolor": "0.6"},
        )
        axis.set_xscale("log")
        axis.set_yscale("log")
        axis.set(
            xlim=limits,
            ylim=limits,
            xlabel="Reported radius (Earth radii, log scale)",
            title=name,
        )
    axes[0].set_ylabel("Out-of-fold predicted radius (Earth radii, log scale)")
    axes[1].legend(frameon=False, loc="lower right")
    fig.tight_layout()
    fig.savefig(FIGURES_DIR / "model_predictions.pdf")
    fig.savefig(FIGURES_DIR / "model_predictions.png")
    plt.close(fig)


def plot_learning_curve(summary: pd.DataFrame) -> None:
    configure_plots()
    fig, axis = plt.subplots(figsize=(6.4, 3.8))
    styles = {
        "Log-linear regression": {"color": "0.55", "marker": "s", "linestyle": "--"},
        "Random forest": {"color": "0.05", "marker": "o", "linestyle": "-"},
    }
    for name, group in summary.groupby("model"):
        group = group.sort_values("training_rows_mean")
        axis.errorbar(
            group["training_rows_mean"],
            group["rmse_mean"],
            yerr=group["rmse_sd"],
            capsize=3,
            linewidth=1.3,
            markersize=4.5,
            label=f"{name} (mean $\\pm$ SD)",
            **styles[name],
        )
    axis.set(
        xlabel="Mean training planets per fold",
        ylabel="Held-out RMSE (Earth radii)",
        title="Host-grouped five-fold learning curve",
    )
    axis.grid(axis="y", color="0.9", linewidth=0.7)
    axis.legend(frameon=False)
    fig.tight_layout()
    fig.savefig(FIGURES_DIR / "learning_curve.pdf")
    fig.savefig(FIGURES_DIR / "learning_curve.png")
    plt.close(fig)


def main() -> None:
    RESULTS_DIR.mkdir(parents=True, exist_ok=True)
    FIGURES_DIR.mkdir(parents=True, exist_ok=True)
    data, quality = load_and_select()
    data.to_csv(RESULTS_DIR / "exoplanets_model_sample.csv", index=False, float_format="%.10g")
    _, spearman = descriptive_analysis(data)
    fold_metrics, inference, importance, predictions = model_analysis(data)
    learning_curve = learning_curve_analysis(data)

    metadata = {
        "seed": SEED,
        "python": platform.python_version(),
        "numpy": np.__version__,
        "pandas": pd.__version__,
        "scikit_learn": sklearn.__version__,
        "data_quality_and_selection": quality,
        "radius_mean_earth_radii": float(data["pl_rade"].mean()),
        "radius_sd_earth_radii": float(data["pl_rade"].std()),
        "radius_median_earth_radii": float(data["pl_rade"].median()),
        "radius_skewness": float(data["pl_rade"].skew()),
        "model_comparison": inference,
        "permutation_importance": [
            {
                "feature": feature,
                "mean_log_rmse_increase_dex": float(row["mean"]),
                "sd_log_rmse_increase_dex": float(row["std"]),
            }
            for feature, row in importance.iterrows()
        ],
        "maximum_learning_curve_training_rows_mean": float(learning_curve["training_rows_mean"].max()),
        "maximum_learning_curve_training_hosts_mean": float(learning_curve["training_hosts_mean"].max()),
    }
    with (RESULTS_DIR / "analysis_summary.json").open("w", encoding="utf-8") as handle:
        json.dump(metadata, handle, indent=2)

    plot_exploration(data, spearman)
    plot_predictions(data, predictions, inference)
    plot_learning_curve(learning_curve)

    print(json.dumps(metadata, indent=2))
    metric_columns = ["rmse_earth_radii", "mae_earth_radii", "r_squared", "log_rmse_dex"]
    print("\nCross-validation summary:\n", fold_metrics.groupby("model")[metric_columns].agg(["mean", "std"]).round(3))
    print("\nPermutation importance:\n", importance.round(4))
    print("\nLearning curve:\n", learning_curve.round(3).to_string(index=False))


if __name__ == "__main__":
    main()
