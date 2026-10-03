import { useState } from 'react'
import { post } from '../../api'
import { Card, Btn, Badge, Table, Modal, Field, Toast, useToast } from '../../ui'

export default function Overview({ building: b, residents, isManager, refresh, show }) {
  const [add, setAdd] = useState(false)
  return (
    <>
      <div className="grid g2">
        <Card title="立项评估信息" extra={isManager && <Btn size="sm" onClick={() => setAdd(true)}>+ 新增住户</Btn>}>
          <dl className="kv">
            <dt>屋顶面积</dt><dd>{b.rooftop_area} m² · 朝向 {b.orientation}</dd>
            <dt>承重评估</dt><dd>{b.load_assessment || '—'}</dd>
            <dt>遮挡情况</dt><dd>{b.shading || '—'}</dd>
            <dt>产权关系</dt><dd>{b.property_rights || '—'}</dd>
            <dt>公共收益规则</dt><dd>{b.public_benefit_rule || '—'}</dd>
            <dt>并网条件</dt><dd>{b.grid_condition || '—'}</dd>
            <dt>施工影响</dt><dd>{b.construction_impact || '—'}</dd>
            <dt>装机规模</dt><dd>{b.total_capacity_kw} kWp · 组件 {b.component_count} 块 · 单块 ¥{b.component_unit_price}</dd>
            <dt>结算参数</dt>
            <dd>
              电价 ¥{b.electricity_price}/kWh · 补贴 ¥{b.subsidy_rate}/kWh ·
              公共比例 {b.public_share_pct}% · 运维 {b.maintenance_fee_pct}% ·
              税费 {b.tax_pct}% · 维修基金 {b.repair_fund_pct}% · 表决门槛 {b.vote_threshold_pct}%
            </dd>
          </dl>
        </Card>

        <Card title={`住户名册(${residents.length} 户)`}>
          <Table head={['房号', '业主', '角色', '面积', '顶楼', '状态']}>
            {residents.map((r) => (
              <tr key={r.id}>
                <td>{r.unit_no}</td>
                <td>{r.owner_name}</td>
                <td>{r.role === 'PROPERTY' ? <Badge tone="violet">物业</Badge> : '住户'}</td>
                <td className="num">{r.area_owned} m²</td>
                <td>{r.is_top_floor ? <Badge tone="amber">顶楼</Badge> : ''}</td>
                <td>{r.active ? <Badge tone="green">在住</Badge> : <Badge>已迁出</Badge>}</td>
              </tr>
            ))}
          </Table>
        </Card>
      </div>
      {add && <AddResident buildingId={b.id} onClose={() => setAdd(false)} onDone={() => { setAdd(false); refresh(); show('住户已新增,初始密码 123456') }} />}
    </>
  )
}

function AddResident({ buildingId, onClose, onDone }) {
  const [f, setF] = useState({ unit_no: '', owner_name: '', phone: '', role: 'RESIDENT', area_owned: 90, is_top_floor: false, password: '123456' })
  const set = (k) => (e) => setF({ ...f, [k]: e.target.type === 'checkbox' ? e.target.checked : e.target.value })
  const submit = async () => {
    try {
      await post(`/api/buildings/${buildingId}/residents`, { ...f, area_owned: Number(f.area_owned) })
      onDone()
    } catch (e) {
      alert(e.message)
    }
  }
  return (
    <Modal title="新增住户" onClose={onClose}>
      <div className="form-row">
        <Field label="房号"><input value={f.unit_no} onChange={set('unit_no')} placeholder="如 701" /></Field>
        <Field label="业主姓名"><input value={f.owner_name} onChange={set('owner_name')} /></Field>
      </div>
      <div className="form-row">
        <Field label="手机号"><input value={f.phone} onChange={set('phone')} /></Field>
        <Field label="角色">
          <select value={f.role} onChange={set('role')}>
            <option value="RESIDENT">普通住户</option>
            <option value="PROPERTY">物业</option>
          </select>
        </Field>
      </div>
      <div className="form-row">
        <Field label="产权面积(m²)"><input type="number" value={f.area_owned} onChange={set('area_owned')} /></Field>
        <Field label="是否顶楼">
          <select value={f.is_top_floor ? '1' : '0'} onChange={(e) => setF({ ...f, is_top_floor: e.target.value === '1' })}>
            <option value="0">否</option>
            <option value="1">是</option>
          </select>
        </Field>
      </div>
      <Field label="初始密码"><input value={f.password} onChange={set('password')} /></Field>
      <div className="modal-ft"><Btn onClick={onClose}>取消</Btn><Btn tone="primary" onClick={submit}>保存</Btn></div>
    </Modal>
  )
}
