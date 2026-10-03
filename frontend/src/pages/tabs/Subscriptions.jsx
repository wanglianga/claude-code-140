import { useState } from 'react'
import { get, post } from '../../api'
import { useLoader, Card, Btn, Badge, Table, Modal, Field, Donut, CNY } from '../../ui'
import { SUBSCRIPTION_STATUS, SUB_STATUS_TONE } from '../../dict'

export default function Subscriptions({ user, buildingId, building: b, residents, isManager, show }) {
  const { data, reload } = useLoader(() => get(`/api/buildings/${buildingId}/subscriptions`), [buildingId])
  const { data: payments, reload: reloadPay } = useLoader(() => get(`/api/buildings/${buildingId}/payments`), [buildingId])
  const [w, setW] = useState(false)
  const canSubscribe = ['VOTING', 'SUBSCRIBED'].includes(b.stage)
  const me = residents.find((r) => r.id === user.resident_id)
  const mySub = (data?.subscriptions || []).find((s) => s.resident_id === user.resident_id)

  return (
    <>
      <div className="grid g3">
        <Card title="组件认购进度">
          <div style={{ display: 'flex', alignItems: 'center', gap: 20 }}>
            <Donut pct={data?.subscribed_pct ?? 0} label="已认购" />
            <div style={{ fontSize: 13, color: '#475569', display: 'grid', gap: 6 }}>
              <div>组件总数:<b>{data?.total_components ?? b.component_count}</b> 块</div>
              <div>已认购:<b style={{ color: 'var(--green-d)' }}>{data?.subscribed_qty ?? 0}</b> 块({data?.subscribed_pct ?? 0}%)</div>
              <div>公共部分:<b>{data?.public_qty ?? 0}</b> 块({data?.public_component_pct ?? 0}%)</div>
              <div>单块价格:¥{CNY(b.component_unit_price)}</div>
            </div>
          </div>
          {!canSubscribe && <div className="note-box">当前阶段为「{b.stage_name}」,认购通道未开放。表决通过进入认购缴费期后方可认购。</div>}
        </Card>

        <Card title="我的认购">
          {user.kind === 'ADMIN' ? <div className="muted">管理员视角,可代住户登记认购。</div>
            : mySub ? (
              <div style={{ display: 'grid', gap: 8, fontSize: 13 }}>
                <div>模式:{mySub.mode === 'SUBSCRIBE' ? '认购组件' : <Badge tone="blue">只参与公共收益分配</Badge>}</div>
                {mySub.mode === 'SUBSCRIBE' && <div>认购组件:<b>{mySub.component_qty}</b> 块 · 占比 {mySub.subscribe_pct}%</div>}
                <div>应缴/已缴:¥{CNY(mySub.amount_paid)}</div>
                <div>状态:<Badge tone={SUB_STATUS_TONE[mySub.status]}>{SUBSCRIPTION_STATUS[mySub.status]}</Badge></div>
                {mySub.status === 'PENDING' && <div style={{ marginTop: 6 }}><Btn tone="primary" onClick={() => pay(mySub.id, mySub.component_qty * b.component_unit_price, show, () => { reload(); reloadPay() })}>立即缴费 ¥{CNY(mySub.component_qty * b.component_unit_price)}</Btn></div>}
              </div>
            ) : (
              <div style={{ display: 'grid', gap: 10 }}>
                <div className="muted">您尚未认购。可认购组件按份享有发电收益,也可选择只参与公共收益分配。</div>
                {canSubscribe && <Btn tone="primary" onClick={() => setW(true)}>{me ? '我要认购' : '登记认购'}</Btn>}
              </div>
            )}
        </Card>

        <Card title="认购与收益规则">
          <div className="list-text">
            1. 每块组件 ¥{b.component_unit_price},按份共有,认购比例 = 认购组件数 / 总组件数。
            {'\n'}2. 月发电净收益的 {100 - b.public_share_pct}% 按认购比例分给认购住户,{b.public_share_pct}% 归公共账户。
            {'\n'}3. 选择"只参与公共收益分配"无需出资,随公共账户年底统一公示/支出受益。
            {'\n'}4. 须在认购缴费期完成缴费,逾期未缴不能进入施工排期。
            {'\n'}5. 房屋转让时可选择权益随房转移或退本金,见"房屋转让"页签。
          </div>
        </Card>
      </div>

      <Card title="认购登记明细" extra={canSubscribe && (isManager || !mySub) && <Btn size="sm" tone="primary" onClick={() => setW(true)}>+ {isManager ? '代住户登记认购' : '我要认购'}</Btn>}>
        <Table head={['房号', '业主', '模式', '组件数', '占比', '金额', '状态', '操作']}>
          {(data?.subscriptions || []).map((s) => (
            <tr key={s.id}>
              <td>{s.unit_no}</td><td>{s.owner_name}</td>
              <td>{s.mode === 'SUBSCRIBE' ? '认购组件' : <Badge tone="blue">公共分配</Badge>}</td>
              <td className="num">{s.component_qty || '—'}</td>
              <td className="num">{s.subscribe_pct ? s.subscribe_pct + '%' : '—'}</td>
              <td className="num">¥{CNY(s.amount_paid)}</td>
              <td><Badge tone={SUB_STATUS_TONE[s.status]}>{SUBSCRIPTION_STATUS[s.status]}</Badge></td>
              <td>
                {s.status === 'PENDING' && (isManager || s.resident_id === user.resident_id) &&
                  <Btn size="sm" tone="primary" onClick={() => pay(s.id, s.component_qty * b.component_unit_price, show, () => { reload(); reloadPay() })}>缴费</Btn>}
              </td>
            </tr>
          ))}
        </Table>
      </Card>

      <Card title="缴款 / 退款 / 收益发放流水">
        <Table head={['时间', '房号', '业主', '类型', '金额', '事由']}>
          {(payments || []).map((p) => (
            <tr key={p.id}>
              <td>{p.created_at}</td><td>{p.unit_no}</td><td>{p.owner_name}</td>
              <td>{p.direction === 'IN' ? <Badge tone="green">认购缴款</Badge> : p.direction === 'OUT' ? <Badge tone="red">退款</Badge> : <Badge tone="blue">收益发放</Badge>}</td>
              <td className="num">¥{CNY(p.amount)}</td><td className="list-text">{p.reason}</td>
            </tr>
          ))}
        </Table>
      </Card>

      {w && <SubscribeWrite buildingId={buildingId} residents={residents} isManager={isManager} b={b}
        onClose={() => setW(false)} onDone={() => { setW(false); reload(); show('认购已登记,请尽快缴费') }} />}
    </>
  )
}

async function pay(sid, due, show, done) {
  try {
    const r = await post(`/api/subscriptions/${sid}/pay`, { amount: due })
    show(`缴费成功 ¥${CNY(r.paid)}`)
    done()
  } catch (e) { show(e.message, 'err') }
}

function SubscribeWrite({ buildingId, residents, isManager, b, onClose, onDone }) {
  const firstResident = residents.find((r) => r.active && r.role === 'RESIDENT')?.id || ''
  const [residentId, setResidentId] = useState(firstResident)
  const [mode, setMode] = useState('SUBSCRIBE')
  const [qty, setQty] = useState(10)
  const amount = qty * b.component_unit_price
  return (
    <Modal title="登记认购" onClose={onClose}>
      {isManager ? (
        <Field label="选择住户">
          <select value={residentId} onChange={(e) => setResidentId(Number(e.target.value))}>
            <option value="">请选择</option>
            {residents.filter((r) => r.active && r.role === 'RESIDENT').map((r) => (
              <option key={r.id} value={r.id}>{r.unit_no} {r.owner_name}{r.is_top_floor ? '(顶楼)' : ''}</option>
            ))}
          </select>
        </Field>
      ) : <Field label="认购住户"><input value="本人" disabled /></Field>}
      <Field label="参与方式">
        <select value={mode} onChange={(e) => setMode(e.target.value)}>
          <option value="SUBSCRIBE">认购组件(按份享有发电收益)</option>
          <option value="PUBLIC_ONLY">只参与公共收益分配(不出资)</option>
        </select>
      </Field>
      {mode === 'SUBSCRIBE' && (
        <>
          <Field label={`认购数量(块,共 ${b.component_count} 块)`}>
            <input type="number" min="1" max={b.component_count} value={qty} onChange={(e) => setQty(Number(e.target.value))} />
          </Field>
          <div className="note-box">应缴金额:<b style={{ fontSize: 15 }}>¥{CNY(amount)}</b> · 认购占比 {((qty / b.component_count) * 100).toFixed(2)}%</div>
        </>
      )}
      <div className="modal-ft"><Btn onClick={onClose}>取消</Btn>
        <Btn tone="primary" onClick={async () => {
          try {
            await post(`/api/buildings/${buildingId}/subscriptions`, { resident_id: residentId, mode, component_qty: mode === 'SUBSCRIBE' ? qty : 0 })
            onDone()
          } catch (e) { alert(e.message) }
        }}>登记认购</Btn></div>
    </Modal>
  )
}
