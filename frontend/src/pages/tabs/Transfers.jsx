import { useState } from 'react'
import { get, post } from '../../api'
import { useLoader, Card, Btn, Badge, Table, Modal, Field, CNY } from '../../ui'
import { TRANSFER_STATUS, TRANSFER_TONE } from '../../dict'

export default function Transfers({ buildingId, residents, isManager, show }) {
  const { data, reload } = useLoader(() => get(`/api/buildings/${buildingId}/transfers`), [buildingId])
  const [w, setW] = useState(false)
  return (
    <>
      <Card title="房屋转让与认购权益处理"
        extra={isManager && <Btn size="sm" tone="primary" onClick={() => setW(true)}>+ 登记转让</Btn>}>
        <Table head={['房号', '原业主', '新业主', '权益随房转移', '历史收益结清', '结清金额', '退款金额', '状态', '处理时间', '操作']}>
          {(data || []).map((t) => (
            <tr key={t.id}>
              <td>{t.from_unit}</td>
              <td>{t.from_name}</td>
              <td>{t.new_owner_name}</td>
              <td>{t.transfer_subscription ? <Badge tone="green">随房转移</Badge> : <Badge tone="amber">不转移,退本金</Badge>}</td>
              <td>{t.settle_history ? <Badge tone="blue">已结清确认</Badge> : <Badge>未结清</Badge>}</td>
              <td className="num">{t.settled_amount ? '¥' + CNY(t.settled_amount) : '—'}</td>
              <td className="num">{t.refund_amount ? '¥' + CNY(t.refund_amount) : '—'}</td>
              <td><Badge tone={TRANSFER_TONE[t.status]}>{TRANSFER_STATUS[t.status]}</Badge></td>
              <td>{t.processed_at || '—'}</td>
              <td>{t.status === 'PENDING' && isManager && <Btn size="sm" tone="primary" onClick={() => process(t.id, show, reload)}>执行处理</Btn>}</td>
            </tr>
          ))}
        </Table>
        <div className="note-box">
          处理规则:① 权益随房转移——认购记录改挂新业主,原账号停用,密码重置为 123456;
          ② 不随房转移——退还原业主已缴认购本金(公共账户出账),认购置为"已退款",新业主可在认购期重新认购;
          ③ 历史收益结清——汇总原业主已确认月度收益(已于发放月入账),出具结清金额。
        </div>
      </Card>
      {w && <TransferWrite buildingId={buildingId} residents={residents} onClose={() => setW(false)}
        onDone={() => { setW(false); reload(); show('转让已登记,待物业/管理员执行') }} />}
    </>
  )
}

async function process(tid, show, reload) {
  if (!confirm('确认执行该转让?涉及账户与认购变更,执行后不可撤销。')) return
  try {
    const r = await post(`/api/transfers/${tid}/process`, {})
    show(`转让完成:结清 ¥${CNY(r.settled_amount)} / 退款 ¥${CNY(r.refund_amount)}`)
    reload()
  } catch (e) { show(e.message, 'err') }
}

function TransferWrite({ buildingId, residents, onClose, onDone }) {
  const active = residents.filter((r) => r.active && r.role === 'RESIDENT')
  const [f, setF] = useState({
    resident_id: active[0]?.id || 0, new_owner_name: '', new_phone: '',
    transfer_subscription: true, settle_history: true, remark: '',
  })
  const set = (k) => (e) => setF({ ...f, [k]: e.target.type === 'checkbox' ? e.target.checked : e.target.value })
  return (
    <Modal title="登记房屋转让" onClose={onClose} wide>
      <div className="form-row">
        <Field label="原业主(房号)">
          <select value={f.resident_id} onChange={(e) => setF({ ...f, resident_id: Number(e.target.value) })}>
            {active.map((r) => <option key={r.id} value={r.id}>{r.unit_no} {r.owner_name}</option>)}
          </select>
        </Field>
        <Field label="新业主姓名"><input value={f.new_owner_name} onChange={(e) => setF({ ...f, new_owner_name: e.target.value })} /></Field>
      </div>
      <Field label="新业主手机号"><input value={f.new_phone} onChange={(e) => setF({ ...f, new_phone: e.target.value })} /></Field>
      <div className="form-row">
        <label className="field"><span>认购权益处理</span>
          <select value={f.transfer_subscription ? '1' : '0'} onChange={(e) => setF({ ...f, transfer_subscription: e.target.value === '1' })}>
            <option value="1">权益随房转移给新业主</option>
            <option value="0">不转移,退还原业主认购本金</option>
          </select>
        </label>
        <label className="field"><span>历史收益</span>
          <select value={f.settle_history ? '1' : '0'} onChange={(e) => setF({ ...f, settle_history: e.target.value === '1' })}>
            <option value="1">历史收益已结清(汇总已发放收益)</option>
            <option value="0">暂不结清</option>
          </select>
        </label>
      </div>
      <Field label="备注"><input value={f.remark} onChange={set('remark')} placeholder="如:二手房交易,买卖合同已备案" /></Field>
      <div className="modal-ft"><Btn onClick={onClose}>取消</Btn>
        <Btn tone="primary" onClick={async () => {
          try {
            await post(`/api/buildings/${buildingId}/transfers`, {
              resident_id: Number(f.resident_id), new_owner_name: f.new_owner_name, new_phone: f.new_phone,
              transfer_subscription: f.transfer_subscription, settle_history: f.settle_history, remark: f.remark,
            })
            onDone()
          } catch (e) { alert(e.message) }
        }}>登记转让</Btn></div>
    </Modal>
  )
}
