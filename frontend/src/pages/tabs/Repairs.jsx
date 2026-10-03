import { useState } from 'react'
import { get, post } from '../../api'
import { useLoader, Card, Btn, Badge, Table, Modal, Field, CNY } from '../../ui'
import { REPAIR_CATEGORY, REPAIR_STATUS, REPAIR_STATUS_TONE, RESPONSIBILITY } from '../../dict'

export default function Repairs({ buildingId, isManager, show }) {
  const { data, reload } = useLoader(() => get(`/api/buildings/${buildingId}/repairs`), [buildingId])
  const { data: devices } = useLoader(() => get(`/api/buildings/${buildingId}/devices`), [buildingId])
  const [w, setW] = useState(false)
  const [resolve, setResolve] = useState(null)

  return (
    <>
      <Card title="维修工单(逆变器故障 / 顶楼漏水等;影响周期收益与责任判定)"
        extra={<Btn size="sm" tone="primary" onClick={() => setW(true)}>+ 报修</Btn>}>
        <Table head={['工单号', '类别', '标题/描述', '设备', '报修人', '责任方', '费用', '影响周期/电量', '赔偿扣减', '状态', '操作']}>
          {(data || []).map((o) => (
            <tr key={o.id}>
              <td style={{ fontWeight: 600 }}>{o.code}</td>
              <td><Badge tone={o.category === 'LEAK' ? 'amber' : o.category === 'INVERTER_FAULT' ? 'red' : 'gray'}>{REPAIR_CATEGORY[o.category]}</Badge></td>
              <td style={{ minWidth: 180 }}>
                <div style={{ fontWeight: 600 }}>{o.title}</div>
                <div className="list-text">{o.description}</div>
                {o.resolution && <div className="note-box" style={{ marginTop: 4 }}>处理:{o.resolution}</div>}
              </td>
              <td>{o.device_code || '—'}</td>
              <td>{o.reporter || '—'}</td>
              <td>{o.status === 'RESOLVED' ? RESPONSIBILITY[o.responsibility] : <span className="muted">待判定</span>}</td>
              <td className="num">{o.cost ? '¥' + CNY(o.cost) : '—'}</td>
              <td>{o.affect_period || '—'}<br /><span className="muted">{o.affected_kwh ? `少发 ${o.affected_kwh} kWh` : ''}</span></td>
              <td className="num">{o.deduction_amount ? <b style={{ color: 'var(--red)' }}>¥{CNY(o.deduction_amount)}</b> : '—'}</td>
              <td><Badge tone={REPAIR_STATUS_TONE[o.status]}>{REPAIR_STATUS[o.status]}</Badge><div className="muted" style={{ fontSize: 11 }}>{o.resolved_at}</div></td>
              <td>{o.status === 'OPEN' && isManager && <Btn size="sm" tone="warn" onClick={() => setResolve(o)}>处理定责</Btn>}</td>
            </tr>
          ))}
        </Table>
        <div className="note-box">
          结算联动:已处理工单若指定「影响周期」,该周期结算将计入维修支出与责任赔偿。赔偿额 = 少发电量 × (电价+补贴) × 责任承担比例;
          物业/公共承担的维修费从公共账户出账,供应商/施工方承担的费用不占用公共资金。
        </div>
      </Card>

      {w && <RepairCreate buildingId={buildingId} devices={devices || []} onClose={() => setW(false)}
        onDone={() => { setW(false); reload(); show('工单已创建') }} />}
      {resolve && <RepairResolve order={resolve} onClose={() => setResolve(null)}
        onDone={() => { setResolve(null); reload(); show('工单已处理定责') }} />}
    </>
  )
}

function RepairCreate({ buildingId, devices, onClose, onDone }) {
  const [f, setF] = useState({ device_id: 0, category: 'INVERTER_FAULT', title: '', description: '' })
  const set = (k) => (e) => setF({ ...f, [k]: e.target.value })
  return (
    <Modal title="创建维修工单" onClose={onClose}>
      <div className="form-row">
        <Field label="问题类别">
          <select value={f.category} onChange={set('category')}>
            <option value="INVERTER_FAULT">逆变器故障</option>
            <option value="LEAK">屋顶漏水</option>
            <option value="OTHER">其他</option>
          </select>
        </Field>
        <Field label="关联设备(漏水等可不选)">
          <select value={f.device_id} onChange={set('device_id')}>
            <option value={0}>不关联设备</option>
            {devices.map((d) => <option key={d.id} value={d.id}>{d.code} {d.name}</option>)}
          </select>
        </Field>
      </div>
      <Field label="标题"><input value={f.title} onChange={set('title')} placeholder="如:2号逆变器停机 / 601室阳台顶角渗水" /></Field>
      <Field label="详细描述"><textarea style={{ minHeight: 110 }} value={f.description} onChange={set('description')} /></Field>
      <div className="modal-ft"><Btn onClick={onClose}>取消</Btn>
        <Btn tone="primary" onClick={async () => {
          try {
            await post(`/api/buildings/${buildingId}/repairs`, { ...f, device_id: Number(f.device_id) })
            onDone()
          } catch (e) { alert(e.message) }
        }}>提交工单</Btn></div>
    </Modal>
  )
}

function RepairResolve({ order, onClose, onDone }) {
  const [f, setF] = useState({
    responsibility: order.category === 'INVERTER_FAULT' ? 'SUPPLIER' : order.category === 'LEAK' ? 'INSTALLER' : 'PUBLIC',
    cost: 0, affect_period: '', affected_kwh: 0, deductible_share: 100, resolution: '',
  })
  const set = (k) => (e) => setF({ ...f, [k]: e.target.value })
  const deduction = Number(f.affected_kwh || 0) * 0.44 * (Number(f.deductible_share || 0) / 100)
  return (
    <Modal title={`处理工单 ${order.code}`} onClose={onClose} wide>
      <div className="form-row">
        <Field label="责任判定">
          <select value={f.responsibility} onChange={set('responsibility')}>
            <option value="SUPPLIER">设备供应商(设备质量)</option>
            <option value="INSTALLER">施工方(安装/防水)</option>
            <option value="PROPERTY">物业(日常维护)</option>
            <option value="RESIDENT">住户(使用不当)</option>
            <option value="PUBLIC">公共(全体业主共担)</option>
          </select>
        </Field>
        <Field label="维修费用(元)">
          <input type="number" value={f.cost} onChange={set('cost')} hint="物业/公共承担时从公共账户出账" />
        </Field>
      </div>
      <div className="form-row">
        <Field label="影响结算周期">
          <input type="month" value={f.affect_period} onChange={set('affect_period')} />
        </Field>
        <Field label="影响/少发电量(kWh)">
          <input type="number" value={f.affected_kwh} onChange={set('affected_kwh')} />
        </Field>
      </div>
      <Field label="责任方收益扣减/赔偿比例(%)" hint="按少发电量×(电价0.39+补贴0.05)×比例计算赔偿,计入当期结算">
        <input type="number" min="0" max="100" value={f.deductible_share} onChange={set('deductible_share')} />
      </Field>
      <Field label="处理结果与责任说明"><textarea style={{ minHeight: 100 }} value={f.resolution} onChange={set('resolution')}
        placeholder="如:经厂家检测为IGBT模块故障,免费更换;承诺72小时内修复并承担发电量损失" /></Field>
      <div className="note-box">预计赔偿金额:<b style={{ color: 'var(--red)' }}>¥{CNY(deduction)}</b>;关联设备将标记为"已修复"。</div>
      <div className="modal-ft"><Btn onClick={onClose}>取消</Btn>
        <Btn tone="primary" onClick={async () => {
          try {
            await post(`/api/repairs/${order.id}/resolve`, {
              ...f, cost: Number(f.cost), affected_kwh: Number(f.affected_kwh), deductible_share: Number(f.deductible_share),
            })
            onDone()
          } catch (e) { alert(e.message) }
        }}>确认处理</Btn></div>
    </Modal>
  )
}
