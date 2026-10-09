import type { ReactNode } from "react";

interface BarItem {
  label: string;
  value: number;
  color?: string;
  group?: string;
}

interface GroupedBar {
  label: string;
  series: { name: string; value: number | null; color: string }[];
}

export function GroupedBarChart({
  title,
  yLabel,
  groups,
  height = 220,
}: {
  title: string;
  yLabel: string;
  groups: GroupedBar[];
  height?: number;
}) {
  const vals = groups.flatMap((g) => g.series.map((s) => s.value ?? 0));
  const max = Math.max(1, ...vals);
  const pad = { t: 28, r: 12, b: 48, l: 48 };
  const w = Math.max(320, groups.length * 88);
  const innerH = height - pad.t - pad.b;
  const innerW = w - pad.l - pad.r;
  const slot = innerW / Math.max(1, groups.length);

  return (
    <figure className="chart">
      <figcaption>{title}</figcaption>
      <svg viewBox={`0 0 ${w} ${height}`} role="img" aria-label={title}>
        <text x={4} y={14} className="chart-axis">
          {yLabel}
        </text>
        {[0, 0.25, 0.5, 0.75, 1].map((f) => {
          const y = pad.t + innerH * (1 - f);
          return (
            <g key={f}>
              <line x1={pad.l} x2={w - pad.r} y1={y} y2={y} className="chart-grid" />
              <text x={pad.l - 6} y={y + 3} textAnchor="end" className="chart-axis">
                {(max * f).toFixed(max >= 10 ? 0 : 1)}
              </text>
            </g>
          );
        })}
        {groups.map((g, gi) => {
          const series = g.series;
          const barW = Math.min(22, (slot * 0.7) / Math.max(1, series.length));
          const groupW = barW * series.length + 4 * (series.length - 1);
          const x0 = pad.l + gi * slot + (slot - groupW) / 2;
          return (
            <g key={g.label}>
              {series.map((s, si) => {
                const v = s.value ?? 0;
                const h = (v / max) * innerH;
                const x = x0 + si * (barW + 4);
                const y = pad.t + innerH - h;
                return (
                  <rect
                    key={s.name}
                    x={x}
                    y={y}
                    width={barW}
                    height={Math.max(0, h)}
                    fill={s.color}
                    opacity={s.value == null ? 0.25 : 1}
                  >
                    <title>{`${g.label} · ${s.name}: ${s.value ?? "n/a"}`}</title>
                  </rect>
                );
              })}
              <text
                x={pad.l + gi * slot + slot / 2}
                y={height - 28}
                textAnchor="middle"
                className="chart-label"
              >
                {g.label}
              </text>
            </g>
          );
        })}
        <line
          x1={pad.l}
          x2={w - pad.r}
          y1={pad.t + innerH}
          y2={pad.t + innerH}
          className="chart-axis-line"
        />
      </svg>
      <div className="legend">
        {groups[0]?.series.map((s) => (
          <span key={s.name} className="legend-item">
            <i style={{ background: s.color }} />
            {s.name}
          </span>
        ))}
      </div>
    </figure>
  );
}

export function LineChart({
  title,
  yLabel,
  xLabel,
  series,
  height = 200,
}: {
  title: string;
  yLabel: string;
  xLabel: string;
  series: { name: string; color: string; points: { x: number; y: number | null }[] }[];
  height?: number;
}) {
  const xs = series.flatMap((s) => s.points.map((p) => p.x));
  const ys = series.flatMap((s) => s.points.map((p) => p.y).filter((v): v is number => v != null));
  if (xs.length === 0 || ys.length === 0) return null;
  const minX = Math.min(...xs);
  const maxX = Math.max(...xs);
  const maxY = Math.max(1, ...ys);
  const pad = { t: 28, r: 16, b: 40, l: 48 };
  const w = 560;
  const innerH = height - pad.t - pad.b;
  const innerW = w - pad.l - pad.r;
  const sx = (x: number) => pad.l + ((x - minX) / Math.max(1e-9, maxX - minX)) * innerW;
  const sy = (y: number) => pad.t + innerH * (1 - y / maxY);

  return (
    <figure className="chart">
      <figcaption>{title}</figcaption>
      <svg viewBox={`0 0 ${w} ${height}`} role="img" aria-label={title}>
        <text x={4} y={14} className="chart-axis">
          {yLabel}
        </text>
        {[0, 0.5, 1].map((f) => {
          const y = pad.t + innerH * (1 - f);
          return (
            <g key={f}>
              <line x1={pad.l} x2={w - pad.r} y1={y} y2={y} className="chart-grid" />
              <text x={pad.l - 6} y={y + 3} textAnchor="end" className="chart-axis">
                {(maxY * f).toFixed(0)}
              </text>
            </g>
          );
        })}
        {series.map((s) => {
          const pts = s.points.filter((p) => p.y != null) as { x: number; y: number }[];
          if (pts.length < 2) return null;
          const d = pts.map((p, i) => `${i === 0 ? "M" : "L"}${sx(p.x)},${sy(p.y)}`).join(" ");
          return <path key={s.name} d={d} fill="none" stroke={s.color} strokeWidth={2} />;
        })}
        <text x={w / 2} y={height - 8} textAnchor="middle" className="chart-axis">
          {xLabel}
        </text>
      </svg>
      <div className="legend">
        {series.map((s) => (
          <span key={s.name} className="legend-item">
            <i style={{ background: s.color }} />
            {s.name}
          </span>
        ))}
      </div>
    </figure>
  );
}

export function HeatCell({ value, label }: { value: number | null; label: string }): ReactNode {
  if (value == null) return <td className="heat muted">—</td>;
  const clamped = Math.max(0, Math.min(1, value));
  return (
    <td
      className="heat"
      title={label}
      style={{ ["--heat" as string]: String(clamped) }}
    >
      {(clamped * 100).toFixed(0)}%
    </td>
  );
}

export function SimpleBars({ title, items, unit }: { title: string; items: BarItem[]; unit: string }) {
  const max = Math.max(1, ...items.map((i) => i.value));
  return (
    <figure className="chart">
      <figcaption>{title}</figcaption>
      <ul className="hbar-list">
        {items.map((it) => (
          <li key={it.label}>
            <span className="hbar-label">{it.label}</span>
            <span className="hbar-track">
              <span
                className="hbar-fill"
                style={{
                  width: `${(it.value / max) * 100}%`,
                  background: it.color ?? "var(--accent)",
                }}
              />
            </span>
            <span className="hbar-val">
              {it.value.toFixed(1)}
              {unit}
            </span>
          </li>
        ))}
      </ul>
    </figure>
  );
}
