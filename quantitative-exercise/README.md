# Quantitative exercise: predicting exoplanet radius

This directory contains a reproducible analysis of a dated NASA Exoplanet
Archive snapshot and a LaTeX report.

## Reproduce the analysis

From this directory, run:

```bash
python3 analysis.py
latexmk -pdf II2202-quantitative-exercise.tex
```

The script validates and filters the archived CSV, keeps host-star systems
together during validation, writes numerical results to `results/`, and exports
each graph as vector PDF and 300 dpi PNG to `figures/`. It uses random seed
`2202` and does not require network access.

Required package versions are recorded in `requirements.txt`.

## Data provenance

`data/exoplanets.csv` is a snapshot of the NASA Exoplanet Archive Planetary
Systems Composite Parameters (`pscomppars`) table, retrieved through its TAP
service on 21 September 2026. The exact query was:

```sql
select pl_name, hostname, disc_year, discoverymethod,
       pl_rade, pl_bmasse, pl_bmassprov, pl_orbper, pl_eqt,
       pl_insol, pl_orbsmax, st_teff, st_rad, st_mass, st_met,
       st_age, sy_dist
from pscomppars
where pl_rade is not null
order by pl_name
```

The CSV response used the TAP endpoint
`https://exoplanetarchive.ipac.caltech.edu/TAP/sync` with `format=csv`.
Its SHA-256 checksum is:

```text
dd7ede0f449172b495db222ed8ea50b95981948ca15952e7eebfe2663a88966e
```

The archive is live and changes as published measurements are added or
revised. Keeping this snapshot and checksum makes the submitted results stable.
The archive and its API are documented at:

- <https://exoplanetarchive.ipac.caltech.edu/>
- <https://exoplanetarchive.ipac.caltech.edu/docs/API_queries.html>
- <https://exoplanetarchive.ipac.caltech.edu/docs/API_PS_columns.html>

## Sample-selection rationale

The analysis starts with 6,316 confirmed planets having a reported radius. It
keeps transit discoveries and accepts only `pl_bmassprov = Mass`. In particular,
it excludes the 2,974 transit-planet masses whose provenance is `M-R
relationship`; using radius-derived mass values to predict radius would be
circular. Complete-case selection over the five predictors and outcome leaves
1,670 planets belonging to 1,304 distinct host stars.
