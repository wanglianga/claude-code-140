import React, { useEffect, useState } from 'react'
import { api, fmtMoney } from '../../lib/api'
import { Badge, RatioBar, handleErr, useToast } from '../ui'

export default function SubscriptionTab({ user, building, reload }) {
  const { notify } = useToast()
  const [subs, setSubs] = useState([])
  const [ratio, setRatio] = useState(10)
  const [asPublic, setAsPublic] = useState(false)
  const isManager = user.role === 'admin' || user.role === 'property'
  const held = subs.filter((s) => s.status !== 'refunded').reduce((a, s) => a + Number(s.ratio || 0), 0)
  const paidHeld = subs.filter((s) => s.status === 'paid').reduce((a, s) => a + Number(s.ratio || 0), 0)

  const load = async () => {
    const r = await api('GET', `/buildings/${building.id}/subscriptions`)
    setSubs(r.items || [])
  }
  useEffect(() => { load() }, [building.id])

  const estimate = () => {
    const r = asPublic ? 0 : ratio
    const count = Math.round((r / 100) * building.panel_total)
    return { count, amount: count * Number(building.unit_price) }
  }
  const subscribe = async () => {
    if (building.stage !== 'subscribing') {
      return notify('当前不在认购缴费阶段（需表决通过后由物业推进）', true)
    }
    try {
      const r = await handleErr(
        api('POST', `/buildings/${building.id}/subscribe`, { type: asPublic ? 'public_only' : 'shareholder', ratio }),
        notify,
      )
      notify(`认购登记成功，应缴 ¥${fmtMoney(r.amount)}（${r.panel_count} 块组件）`)
      load()
    } catch { /* toast */ }
  }
  const pay = async (s) => {
    try {
      await handleErr(api('POST', `/subscriptions/${s.id}/pay`), notify)
      notify('缴费成功，认购款已入公共账户')
      load(); reload()
    } catch { /* toast */ }
  }
  const refund = async (s) => {
    if (!window.confirm(`确认对 ${s.name} 的认购办理退购退款 ¥${fmtMoney(s.amount)}？`)) return
    try {
      const r = await handleErr(api('POST', `/subscriptions/${s.id}/refund`), notify)
      notify(`退购完成，退款 ¥${fmtMoney(r.refunded)}`)
      load(); reload()
    } catch { /* toast */ }
  }

  const est = estimate()

  return (
    <>
      <div className="card">
        <div className="card-title">
          <h2>组件认购</h2>
          <Badge value={building.stage} />
        </div>
        <dl className="kv">
          <dt>组件总数</dt><dd>{building.panel_total} 块 / {building.unit_price} 元每块</dd>
          <dt>有效认购</dt><dd>{Number(held).toFixed(2)}%（其中已缴费 {Number(paidHeld).toFixed(2)}%）</dd>
        </dl>
        <RatioBar ratio={held} />
        <div style={{ height: 10 }} />
        {building.stage === 'subscribing' ? (
          <div className="form-row" style={{ marginTop: 8 }}>
            <label className="field">
              <span>我要认购的组件比例 %</span>
              <input type="number" min="1" max="100" value={ratio} onChange={(e) => setRatio(Number(e.target.value))} disabled={asPublic} />
            </label>
            <label className="field">
              <span>参与方式</span>
              <select value={asPublic ? 'public' : 'share'} onChange={(e) => setAsPublic(e.target.value === 'public')}>
                <option value="share">认购组件（按份额分红）</option>
                <option value="public">只参与公共收益分配（不出资）</option>
              </select>
            </label>
            <button className="primary" onClick={subscribe}>登记认购</button>
          </div>
        ) : (
          <div className="muted-box" style={{ marginTop: 10 }}>
            当前为「{building.stage}」阶段。并网验收前如调整方案，已缴认购款可由物业办理退购退款。
          </div>
        )}
        {!asPublic && building.stage === 'subscribing' && (
          <div className="small muted" style={{ marginTop: 6 }}>
            预计认购 {est.count} 块，需缴 ¥{fmtMoney(est.amount)}；剩余可认购 {(100 - held).toFixed(2)}%
          </div>
        )}
      </div>

      <div className="card">
        <h2>认购明细</h2>
        <table>
          <thead>
            <tr>
              <th>住户</th><th>房号</th><th>类型</th><th className="num">比例</th>
              <th className="num">组件数</th><th className="num">金额</th><th>状态</th><th>操作</th>
            </tr>
          </thead>
          <tbody>
            {subs.map((s) => (
              <tr key={s.id}>
                <td>{s.name}</td><td>{s.unit_no}</td>
                <td><Badge value={s.type} /></td>
                <td className="num">{s.type === 'public_only' ? '—' : Number(s.ratio).toFixed(2) + '%'}</td>
                <td className="num">{s.panel_count || '—'}</td>
                <td className="num">{fmtMoney(s.amount)}</td>
                <td><Badge value={s.status} /></td>
                <td>
                  <div className="btn-row">
                    {s.status === 'pending' && (s.user_id === user.id || isManager) && (
                      <button className="sm primary" onClick={() => pay(s)}>确认缴费</button>
                    )}
                    {isManager && s.status !== 'refunded' && building.stage !== 'operating' && (
                      <button className="sm danger" onClick={() => refund(s)}>退购退款</button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {subs.length === 0 && <div className="list-empty">暂无认购记录</div>}
      </div>
    </>
  )
}
