import { useState } from 'react'
import { get, post } from '../../api'
import { useLoader, Card, Btn, Badge, Table, Modal, Field } from '../../ui'
import { SCHEDULE_KIND, SCHEDULE_KIND_TONE, DEVICE_STATUS, DEVICE_STATUS_TONE } from '../../dict'

export default function ScheduleDevices({ buildingId, building: b, isManager, show }) {
  const [pane, setPane] = useState('sched')
  return (
    <>
      <div className="tabs">
        <button className={pane === 'sched' ? 'on' : ''} onClick={() => setPane('sched')}>施工排期与变更留痕</button>
        <button className={pane === 'dev' ? 'on' : ''} onClick={() => setPane('dev')}>逆变器/组件组</button>
      </div>
      {pane === 'sched' && <SchedulePane buildingId={buildingId} isManager={isManager} show={show} />}
      {pane === 'dev' && <DevicePane buildingId={buildingId} b={b} isManager={isManager} show={show} />}
    </>
  )
}

function SchedulePane({ buildingId, isManager, show }) {
  const { data, reload } = useLoader(() => get(`/api/buildings/${buildingId}/schedule`), [buildingId])
  const [w, setW] = useState(false)
  return (
    <Card title="排期事件(供应延迟 / 并网时间变化均可留痕)"
      extra={isManager && <Btn size="sm" tone="primary" onClick={() => setW(true)}>+ 新增事件</Btn>}>
      <Table head={['类型', '标题', '说明', '计划日期', '实际日期', '登记时间']}>
        {(data || []).map((e) => (
          <tr key={e.id}>
            <td><Badge tone={SCHEDULE_KIND_TONE[e.kind] || 'gray'}>{SCHEDULE_KIND[e.kind] || e.kind}</Badge></td>
            <td style={{ fontWeight: 600 }}>{e.title}</td>
            <td className="list-text">{e.detail}</td>
            <td>{e.planned_on || '—'}</td>
            <td>{e.actual_on || '—'}</td>
            <td>{e.created_at}</td>
          </tr>
        ))}
      </Table>
      {w && <EventWrite buildingId={buildingId} onClose={() => setW(false)} onDone={() => { setW(false); reload(); show('事件已登记') }} />}
    </Card>
  )
}

function EventWrite({ buildingId, onClose, onDone }) {
  const [f, setF] = useState({ kind: 'PLAN', title: '', detail: '', planned_on: '', actual_on: '' })
  const set = (k) => (e) => setF({ ...f, [k]: e.target.value })
  return (
    <Modal title="登记排期事件" onClose={onClose}>
      <div className="form-row">
        <Field label="类型">
          <select value={f.kind} onChange={set('kind')}>
            <option value="PLAN">计划</option>
            <option value="DELAY">设备供应延迟</option>
            <option value="GRID_CHANGE">电网并网时间变化</option>
            <option value="MILESTONE">里程碑/阶段调整</option>
          </select>
        </Field>
        <Field label="标题"><input value={f.title} onChange={set('title')} placeholder="如:组件到货延迟2周" /></Field>
      </div>
      <Field label="说明(对住户的影响与安排)"><textarea value={f.detail} onChange={set('detail')} /></Field>
      <div className="form-row">
        <Field label="计划日期"><input type="date" value={f.planned_on} onChange={set('planned_on')} /></Field>
        <Field label="实际日期"><input type="date" value={f.actual_on} onChange={set('actual_on')} /></Field>
      </div>
      <div className="modal-ft"><Btn onClick={onClose}>取消</Btn>
        <Btn tone="primary" onClick={async () => {
          try { await post(`/api/buildings/${buildingId}/schedule`, f); onDone() }
          catch (e) { alert(e.message) }
        }}>保存</Btn></div>
    </Modal>
  )
}

function DevicePane({ buildingId, b, isManager, show }) {
  const { data, reload } = useLoader(() => get(`/api/buildings/${buildingId}/devices`), [buildingId])
  const [w, setW] = useState(false)
  const canEdit = ['SCHEDULED', 'GRID_CONNECTED', 'OPERATING'].includes(b.stage)
  return (
    <Card title="逆变器与组件组(发电量按设备归集)"
      extra={isManager && canEdit && <Btn size="sm" tone="primary" onClick={() => setW(true)}>+ 登记设备</Btn>}>
      {!canEdit && <div className="note-box">进入施工排期阶段后可登记设备,并网验收前需完成全部逆变器登记。</div>}
      <Table head={['设备编码', '名称', '类型', '组件区间', '容量(kW)', '状态', '累计发电(kWh)']}>
        {(data || []).map((d) => (
          <tr key={d.id}>
            <td style={{ fontWeight: 600 }}>{d.code}</td>
            <td>{d.name}</td>
            <td>{d.device_type === 'INVERTER' ? '逆变器' : d.device_type}</td>
            <td>#{d.component_from} ~ #{d.component_to} <span className="muted">({d.component_to - d.component_from + 1} 块)</span></td>
            <td className="num">{d.capacity_kw}</td>
            <td><Badge tone={DEVICE_STATUS_TONE[d.status]}>{DEVICE_STATUS[d.status]}</Badge></td>
            <td className="num">{d.total_kwh}</td>
          </tr>
        ))}
      </Table>
      {w && <DeviceWrite buildingId={buildingId} b={b} onClose={() => setW(false)} onDone={() => { setW(false); reload(); show('设备已登记') }} />}
    </Card>
  )
}

function DeviceWrite({ buildingId, b, onClose, onDone }) {
  const [f, setF] = useState({ code: '', name: '', device_type: 'INVERTER', component_from: 1, component_to: 50, capacity_kw: 25 })
  const set = (k) => (e) => setF({ ...f, [k]: e.target.value })
  return (
    <Modal title="登记逆变器/组件组" onClose={onClose}>
      <div className="form-row">
        <Field label="设备编码"><input value={f.code} onChange={set('code')} placeholder="INV-01" /></Field>
        <Field label="设备名称"><input value={f.name} onChange={set('name')} placeholder="1号组串式逆变器" /></Field>
      </div>
      <div className="form-row">
        <Field label="组件起始编号"><input type="number" value={f.component_from} onChange={set('component_from')} /></Field>
        <Field label="组件结束编号"><input type="number" value={f.component_to} onChange={set('component_to')} /></Field>
      </div>
      <div className="form-row">
        <Field label="容量(kW)"><input type="number" value={f.capacity_kw} onChange={set('capacity_kw')} /></Field>
        <Field label="类型">
          <select value={f.device_type} onChange={set('device_type')}><option value="INVERTER">逆变器</option></select>
        </Field>
      </div>
      <div className="note-box">组件区间合计 {Math.max(0, Number(f.component_to) - Number(f.component_from) + 1)} 块,总容量不应超过 {b.total_capacity_kw} kWp。</div>
      <div className="modal-ft"><Btn onClick={onClose}>取消</Btn>
        <Btn tone="primary" onClick={async () => {
          try {
            await post(`/api/buildings/${buildingId}/devices`, {
              ...f, component_from: Number(f.component_from), component_to: Number(f.component_to), capacity_kw: Number(f.capacity_kw),
            })
            onDone()
          } catch (e) { alert(e.message) }
        }}>保存</Btn></div>
    </Modal>
  )
}
