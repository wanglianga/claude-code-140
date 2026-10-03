import React, { useEffect, useState } from 'react'
import { api, fmtNum } from '../../lib/api'
import { Badge, handleErr, useToast } from '../ui'

export default function GenerationTab({ user, building }) {
  const { notify } = useToast()
  const isManager = user.role === 'admin' || user.role === 'property'
  const [inverters, setInverters] = useState([])
  const [records, setRecords] = useState([])
  const [monthTotals, setMonthTotals] = useState({})
  const [alerts, setAlerts] = useState([])
  const [form, setForm] = useState({ period_month: '2026-09', inverter_id: 0, kwh: 0, note: '' })
  const [alertForm, setAlertForm] = useState({ period_month: '2026-09', inverter_id: 0, reason: '' })

  const load = async () => {
    const [iv, g, al] = await Promise.all([
      api('GET', `/buildings/${building.id}/inverters`),
      api('GET', `/buildings/${building.id}/generation`),
      api('GET', `/buildings/${building.id}/alerts`),
    ])
    setInverters(iv.items || [])
    setRecords(g.items || [])
    setMonthTotals(g.month_totals || {})
    setAlerts(al.items || [])
  }
  useEffect(() => { load() }, [building.id])

  const upload = async () => {
    try {
      await handleErr(api('POST', `/buildings/${building.id}/generation`, form), notify)
      notify('发电量已录入/更新')
      load()
    } catch { /* toast */ }
  }
  const markFault = async (inv) => {
    const note = window.prompt(`标记 ${inv.name} 故障，填写故障说明：`, inv.fault_note || '')
    if (note === null) return
    try {
      await handleErr(api('POST', `/inverters/${inv.id}/fault`, { note }), notify)
      notify('设备已标记故障')
      load()
    } catch { /* toast */ }
  }
  const resolveFault = async (inv) => {
    try {
      await handleErr(api('POST', `/inverters/${inv.id}/resolve`), notify)
      notify('设备已恢复正常')
      load()
    } catch { /* toast */ }
  }
  const raiseAlert = async () => {
    if (!alertForm.reason) return notify('请描述异常情况', true)
    try {
      await handleErr(api('POST', `/buildings/${building.id}/alerts`, alertForm), notify)
      notify('异常已上报，等待物业解释')
      setAlertForm({ ...alertForm, reason: '' })
      load()
    } catch { /* toast */ }
  }
  const explain = async (a) => {
    const explanation = window.prompt('对该发电异常的解释（如限电、检修、遮挡、设备故障等）：', a.explanation)
    if (!explanation) return
    try {
      await handleErr(api('POST', `/alerts/${a.id}/explain`, { explanation }), notify)
      notify('解释已发布')
      load()
    } catch { /* toast */ }
  }

  const months = [...new Set(records.map((r) => r.period_month))].sort().reverse()

  return (
    <>
      <div className="card">
        <h2>逆变器 / 组串</h2>
        <table>
          <thead><tr><th>名称</th><th className="num">容量 kW</th><th className="num">装机占比</th><th>状态</th><th>故障说明</th>{isManager && <th>操作</th>}</tr></thead>
          <tbody>
            {inverters.map((iv) => (
              <tr key={iv.id}>
                <td>{iv.name}</td>
                <td className="num">{fmtNum(iv.capacity_kw, 1)}</td>
                <td className="num">{Number(iv.capacity_ratio).toFixed(1)}%</td>
                <td><Badge value={iv.status} /></td>
                <td className="small">{iv.fault_note || '—'}</td>
                {isManager && (
                  <td>
                    <div className="btn-row">
                      {iv.status === 'normal'
                        ? <button className="sm danger" onClick={() => markFault(iv)}>报故障</button>
                        : <button className="sm primary" onClick={() => resolveFault(iv)}>恢复</button>}
                    </div>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
        {inverters.length === 0 && <div className="list-empty">并网验收前由物业登记逆变器信息</div>}
      </div>

      {isManager && (
        <div className="card">
          <h2>录入月度发电量</h2>
          <p className="muted small">按逆变器分表录入，或把逆变器留空录入楼总表读数；同月重复录入会覆盖更新。</p>
          <div className="form-row">
            <label className="field">
              <span>月份</span>
              <input value={form.period_month} onChange={(e) => setForm({ ...form, period_month: e.target.value })} placeholder="YYYY-MM" />
            </label>
            <label className="field">
              <span>计量表</span>
              <select value={form.inverter_id} onChange={(e) => setForm({ ...form, inverter_id: Number(e.target.value) })}>
                <option value={0}>楼总表</option>
                {inverters.map((iv) => <option key={iv.id} value={iv.id}>{iv.name}</option>)}
              </select>
            </label>
            <label className="field">
              <span>发电量 kWh</span>
              <input type="number" value={form.kwh} onChange={(e) => setForm({ ...form, kwh: Number(e.target.value) })} />
            </label>
            <label className="field">
              <span>备注</span>
              <input value={form.note} onChange={(e) => setForm({ ...form, note: e.target.value })} />
            </label>
            <button className="primary" onClick={upload}>录入</button>
          </div>
        </div>
      )}

      <div className="card">
        <h2>发电计量记录</h2>
        {months.map((m) => (
          <div key={m} className="muted-box" style={{ marginBottom: 10 }}>
            <div className="card-title" style={{ marginBottom: 6 }}>
              <h3 style={{ margin: 0 }}>{m}</h3>
              <b>分表合计 {fmtNum(records.filter((r) => r.period_month === m).reduce((a, r) => a + Number(r.kwh), 0))} kWh
                {monthTotals[m] ? ` · 总表 ${fmtNum(monthTotals[m])} kWh` : ''}
              </b>
            </div>
            <table>
              <tbody>
                {records.filter((r) => r.period_month === m).map((r) => (
                  <tr key={r.inverter_id + m}>
                    <td>{r.inverter_name}</td>
                    <td className="num">{fmtNum(r.kwh)} kWh</td>
                    <td className="small muted">{r.note}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ))}
        {records.length === 0 && <div className="list-empty">暂无发电记录</div>}
      </div>

      <div className="card">
        <h2>发电异常与解释</h2>
        <div className="form-row">
          <label className="field">
            <span>月份</span>
            <input value={alertForm.period_month} onChange={(e) => setAlertForm({ ...alertForm, period_month: e.target.value })} placeholder="YYYY-MM" />
          </label>
          <label className="field">
            <span>相关设备</span>
            <select value={alertForm.inverter_id} onChange={(e) => setAlertForm({ ...alertForm, inverter_id: Number(e.target.value) })}>
              <option value={0}>楼总表/整楼</option>
              {inverters.map((iv) => <option key={iv.id} value={iv.id}>{iv.name}</option>)}
            </select>
          </label>
          <label className="field" style={{ flex: 2 }}>
            <span>异常描述（任何住户均可上报）</span>
            <input value={alertForm.reason} onChange={(e) => setAlertForm({ ...alertForm, reason: e.target.value })} />
          </label>
          <button className="primary" onClick={raiseAlert}>上报异常</button>
        </div>
        <table style={{ marginTop: 10 }}>
          <thead><tr><th>月份</th><th>设备</th><th>异常</th><th>物业解释</th><th>状态</th>{isManager && <th>操作</th>}</tr></thead>
          <tbody>
            {alerts.map((a) => (
              <tr key={a.id}>
                <td>{a.period_month}</td>
                <td>{a.inverter_name}</td>
                <td>{a.reason}</td>
                <td className="small">{a.explanation || <span className="muted">待解释</span>}</td>
                <td><Badge value={a.status} /></td>
                {isManager && <td>{a.status === 'open' && <button className="sm" onClick={() => explain(a)}>填写解释</button>}</td>}
              </tr>
            ))}
          </tbody>
        </table>
        {alerts.length === 0 && <div className="list-empty">暂无异常记录</div>}
      </div>
    </>
  )
}
