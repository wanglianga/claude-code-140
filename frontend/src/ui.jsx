import { useEffect, useRef, useState } from 'react'

export const CNY = (v) =>
  (Number(v) || 0).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
export const NUM = (v, d = 1) =>
  (Number(v) || 0).toLocaleString('zh-CN', { minimumFractionDigits: d, maximumFractionDigits: d })

export function Card({ title, extra, children, className = '' }) {
  return (
    <div className={'card ' + className}>
      {title && (
        <div className="card-hd">
          <h3>{title}</h3>
          <div className="card-extra">{extra}</div>
        </div>
      )}
      <div className="card-bd">{children}</div>
    </div>
  )
}

export function Badge({ tone = 'gray', children }) {
  return <span className={'badge b-' + tone}>{children}</span>
}

export const STAGE_TONE = {
  INIT: 'gray', VOTING: 'amber', SUBSCRIBED: 'blue',
  SCHEDULED: 'cyan', GRID_CONNECTED: 'violet', OPERATING: 'green',
}

export function Btn({ children, onClick, tone, size, type = 'button', disabled, title }) {
  return (
    <button
      type={type}
      title={title}
      disabled={disabled}
      className={'btn' + (tone ? ' btn-' + tone : '') + (size ? ' btn-' + size : '')}
      onClick={onClick}
    >
      {children}
    </button>
  )
}

export function Field({ label, children, hint }) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
      {hint && <em className="hint">{hint}</em>}
    </label>
  )
}

export function Modal({ title, onClose, children, wide }) {
  return (
    <div className="modal-mask" onClick={onClose}>
      <div className={'modal' + (wide ? ' modal-wide' : '')} onClick={(e) => e.stopPropagation()}>
        <div className="modal-hd">
          <h3>{title}</h3>
          <button className="modal-x" onClick={onClose}>×</button>
        </div>
        <div className="modal-bd">{children}</div>
      </div>
    </div>
  )
}

export function Toast({ msg }) {
  if (!msg) return null
  return <div className={'toast toast-' + msg.tone}>{msg.text}</div>
}

export function useToast() {
  const [msg, setMsg] = useState(null)
  const timer = useRef()
  const show = (text, tone = 'ok') => {
    setMsg({ text, tone })
    clearTimeout(timer.current)
    timer.current = setTimeout(() => setMsg(null), 2800)
  }
  return [msg, show]
}

export function Table({ head, children, empty = '暂无数据' }) {
  return (
    <div className="table-wrap">
      <table className="tbl">
        <thead>
          <tr>
            {head.map((h, i) => (
              <th key={i}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {children?.length ? (
            children
          ) : (
            <tr>
              <td colSpan={head.length} className="empty">{empty}</td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

export function useLoader(fn, deps = []) {
  const [data, setData] = useState(null)
  const [err, setErr] = useState('')
  const [loading, setLoading] = useState(true)
  const load = () => {
    setLoading(true)
    Promise.resolve(fn())
      .then(setData)
      .catch((e) => setErr(e.message))
      .finally(() => setLoading(false))
  }
  useEffect(load, deps) // eslint-disable-line
  return { data, err, loading, reload: load, setData }
}

// 简易柱状图(SVG)
export function Bars({ data, height = 180, unit = '', color = '#2f9e6e', format = (v) => v }) {
  if (!data?.length) return <div className="empty" style={{ padding: 24 }}>暂无数据</div>
  const max = Math.max(...data.map((d) => d.value), 1)
  const bw = Math.min(46, 360 / data.length - 6)
  return (
    <div className="bars">
      <svg width="100%" height={height} viewBox={`0 0 ${data.length * (bw + 14) + 8} ${height}`}>
        {[0.25, 0.5, 0.75, 1].map((g) => (
          <line key={g} x1="0" x2="100%" y1={height - 28 - (height - 46) * g}
            y2={height - 28 - (height - 46) * g} stroke="#eef1f4" strokeWidth="1" />
        ))}
        {data.map((d, i) => {
          const h = Math.max(2, ((height - 46) * d.value) / max)
          const x = 8 + i * (bw + 14)
          const y = height - 28 - h
          return (
            <g key={i}>
              <rect x={x} y={y} width={bw} height={h} rx="3" fill={d.color || color} opacity={d.dim ? 0.45 : 1} />
              <text x={x + bw / 2} y={y - 4} textAnchor="middle" fontSize="9" fill="#475569">{format(d.value)}</text>
              <text x={x + bw / 2} y={height - 12} textAnchor="middle" fontSize="9" fill="#64748b">{d.label}</text>
            </g>
          )
        })}
      </svg>
      {unit && <div className="bars-unit">单位:{unit}</div>}
    </div>
  )
}

// 环形占比
export function Donut({ pct, size = 120, label }) {
  const r = 48
  const c = 2 * Math.PI * r
  return (
    <div className="donut">
      <svg width={size} height={size} viewBox="0 0 120 120">
        <circle cx="60" cy="60" r={r} fill="none" stroke="#edf0f3" strokeWidth="14" />
        <circle cx="60" cy="60" r={r} fill="none" stroke="#2f9e6e" strokeWidth="14"
          strokeLinecap="round" strokeDasharray={`${(Math.min(100, pct) / 100) * c} ${c}`}
          transform="rotate(-90 60 60)" />
        <text x="60" y="58" textAnchor="middle" fontSize="20" fontWeight="700" fill="#1e293b">{pct}%</text>
        <text x="60" y="76" textAnchor="middle" fontSize="10" fill="#64748b">{label || '已认购'}</text>
      </svg>
    </div>
  )
}
