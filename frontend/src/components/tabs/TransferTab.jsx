import React, { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { Badge, handleErr, useToast } from '../ui'

export default function TransferTab({ user, building }) {
  const { notify } = useToast()
  const isManager = user.role === 'admin' || user.role === 'property'
  const [subs, setSubs] = useState([])
  const [transfers, setTransfers] = useState([])
  const [users, setUsers] = useState([])
  const [form, setForm] = useState({
    subscription_id: 0, to_user_id: 0, to_name: '', to_unit: '',
    rights_transfer: true, refund_requested: false, memo: '',
  })

  const load = async () => {
    const [s, t, u] = await Promise.all([
      api('GET', `/buildings/${building.id}/subscriptions`),
      api('GET', `/buildings/${building.id}/transfers`),
      isManager ? api('GET', '/users') : Promise.resolve({ items: [] }),
    ])
    setSubs(s.items || [])
    setTransfers(t.items || [])
    setUsers(u.items || [])
  }
  useEffect(() => { load() }, [building.id])

  const paidSubs = subs.filter((x) => x.status === 'paid' && x.type === 'shareholder')

  const submit = async () => {
    if (!form.subscription_id) return notify('请选择要办理的认购记录', true)
    if (form.rights_transfer && !form.to_user_id && !form.to_name) {
      return notify('权益随房转移需选择新业主（已注册用户）或填写新业主信息', true)
    }
    try {
      await handleErr(api('POST', `/buildings/${building.id}/transfers`, form), notify)
      notify('转让/退购申请已提交，等待物业确认')
      setForm({ subscription_id: 0, to_user_id: 0, to_name: '', to_unit: '', rights_transfer: true, refund_requested: false, memo: '' })
      load()
    } catch { /* toast */ }
  }

  const confirm = async (t) => {
    if (!window.confirm('确认前请核对历史收益已结清。确认办理？')) return
    try {
      await handleErr(api('POST', `/transfers/${t.id}/confirm`), notify)
      notify('办理完成')
      load()
    } catch { /* toast */ }
  }
  const reject = async (t) => {
    const memo = window.prompt('驳回原因：', '')
    if (memo === null) return
    try {
      await handleErr(api('POST', `/transfers/${t.id}/reject`, { memo }), notify)
      load()
    } catch { /* toast */ }
  }

  return (
    <>
      <div className="card">
        <h2>房屋转让与认购权益处理</h2>
        <div className="muted-box" style={{ marginBottom: 12 }}>
          规则：①<b>权益随房转移</b>——认购过户给新业主，历史月份收益仍归原业主（已结算入其余额），后续月份按新份额分给新业主；
          ②<b>退购</b>——认购不随房转移时按退购处理，退还认购款（并网验收后原则上不退，需物业审核）；
          ③办理前系统会校验是否存在<b>未定稿的月度结算</b>，确保历史收益先结清。
        </div>
        <div className="form-row">
          <label className="field">
            <span>认购记录</span>
            <select value={form.subscription_id} onChange={(e) => setForm({ ...form, subscription_id: Number(e.target.value) })}>
              <option value={0}>请选择…</option>
              {paidSubs.map((s) => (
                <option key={s.id} value={s.id}>{s.name}（{s.unit_no}）· {Number(s.ratio).toFixed(1)}% · ¥{s.amount}</option>
              ))}
            </select>
          </label>
          <label className="field">
            <span>处理方式</span>
            <select
              value={form.rights_transfer ? 'transfer' : 'refund'}
              onChange={(e) => {
                const tr = e.target.value === 'transfer'
                setForm({ ...form, rights_transfer: tr, refund_requested: !tr })
              }}
            >
              <option value="transfer">权益随房屋转移给新业主</option>
              <option value="refund">不转移，办理退购退款</option>
            </select>
          </label>
          {form.rights_transfer && (
            <label className="field">
              <span>新业主（已注册住户）</span>
              <select value={form.to_user_id} onChange={(e) => setForm({ ...form, to_user_id: Number(e.target.value) })}>
                <option value={0}>请选择或在右侧填写…</option>
                {users.filter((u) => u.role === 'resident').map((u) => (
                  <option key={u.id} value={u.id}>{u.name}（{u.unit_no || u.phone}）</option>
                ))}
              </select>
            </label>
          )}
        </div>
        {form.rights_transfer && (
          <div className="form-row">
            <label className="field"><span>新业主姓名（未注册时）</span><input value={form.to_name} onChange={(e) => setForm({ ...form, to_name: e.target.value })} /></label>
            <label className="field"><span>新房号</span><input value={form.to_unit} onChange={(e) => setForm({ ...form, to_unit: e.target.value })} /></label>
          </div>
        )}
        <label className="field"><span>备注</span><input value={form.memo} onChange={(e) => setForm({ ...form, memo: e.target.value })} /></label>
        <button className="primary" onClick={submit}>提交申请</button>
        {paidSubs.length === 0 && <div className="small muted" style={{ marginTop: 8 }}>当前没有可办理的已缴费认购。</div>}
      </div>

      <div className="card">
        <h2>转让 / 退购记录</h2>
        <table>
          <thead>
            <tr>
              <th>申请时间</th><th>原业主</th><th>新业主</th><th>方式</th><th>历史收益</th>
              <th>状态</th><th>备注</th>{isManager && <th>操作</th>}
            </tr>
          </thead>
          <tbody>
            {transfers.map((t) => (
              <tr key={t.id}>
                <td className="small">{new Date(t.created_at).toLocaleDateString('zh-CN')}</td>
                <td>{t.from_name}<div className="small muted">{t.from_unit}</div></td>
                <td>{t.rights_transfer ? (t.to_name || '—') : <span className="muted">退购，无新业主</span>}<div className="small muted">{t.to_unit}</div></td>
                <td>{t.rights_transfer ? <Badge value="transferred" /> : <Badge value="refunded" />}</td>
                <td>{t.history_settled ? <span className="badge green">已结清</span> : <span className="badge amber">待结清</span>}</td>
                <td><Badge value={t.status === 'confirmed' ? 'paid' : t.status === 'rejected' ? 'refunded' : 'pending'} /><span className="small" style={{ marginLeft: 4 }}>{t.status === 'paid' ? '' : t.status === 'confirmed' ? '已确认' : t.status === 'pending' ? '待确认' : '已驳回'}</span></td>
                <td className="small muted">{t.memo}</td>
                {isManager && (
                  <td>
                    {t.status === 'pending' && (
                      <div className="btn-row">
                        <button className="sm primary" onClick={() => confirm(t)}>确认办理</button>
                        <button className="sm danger" onClick={() => reject(t)}>驳回</button>
                      </div>
                    )}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
        {transfers.length === 0 && <div className="list-empty">暂无转让/退购记录</div>}
      </div>
    </>
  )
}
