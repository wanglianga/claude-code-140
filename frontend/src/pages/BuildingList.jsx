import React, { useEffect, useState } from 'react'
import { api, fmtMoney, fmtNum, stageLabel } from '../lib/api'
import { Badge, handleErr, useToast } from '../components/ui'

export default function BuildingList({ user, onOpen }) {
  const { notify } = useToast()
  const [items, setItems] = useState([])
  const [showForm, setShowForm] = useState(false)
  const isManager = user.role === 'admin' || user.role === 'property'

  const load = async () => {
    try {
      const r = await handleErr(api('GET', '/buildings'), notify)
      setItems(r.items)
    } catch { /* toast shown */ }
  }
  useEffect(() => { load() }, [])

  return (
    <div className="container">
      <div className="card-title">
        <h1>光伏楼栋</h1>
        {isManager && (
          <div className="btn-row">
            <button className="primary" onClick={() => setShowForm((v) => !v)}>
              {showForm ? '取消' : '+ 新增楼栋建档'}
            </button>
          </div>
        )}
      </div>

      {showForm && <CreateBuildingForm onCreated={() => { setShowForm(false); load() }} />}

      <div className="grid grid-2">
        {items.map((b) => (
          <div className="card" key={b.id} style={{ cursor: 'pointer' }} onClick={() => onOpen(b.id)}>
            <div className="card-title">
              <h2 style={{ margin: 0 }}>{b.name}</h2>
              <Badge value={b.stage} />
            </div>
            <div className="muted small" style={{ marginBottom: 10 }}>{b.address}</div>
            <div className="grid grid-4">
              <div>
                <div className="small muted">装机容量</div>
                <b>{fmtNum(b.total_capacity_kw)} kW</b>
              </div>
              <div>
                <div className="small muted">组件数</div>
                <b>{b.panel_total} 块</b>
              </div>
              <div>
                <div className="small muted">已认购</div>
                <b>{Number(b.subscribed_ratio || 0).toFixed(1)}%</b>
              </div>
              <div>
                <div className="small muted">单价</div>
                <b>{fmtMoney(b.unit_price)} 元</b>
              </div>
            </div>
            <div className="small muted" style={{ marginTop: 10 }}>
              当前阶段：{stageLabel(b.stage)} · 朝向 {b.orientation || '—'} · 屋顶 {fmtNum(b.roof_area)} m²
            </div>
          </div>
        ))}
      </div>
      {items.length === 0 && <div className="list-empty">暂无楼栋，点击右上角新增</div>}
    </div>
  )
}

function CreateBuildingForm({ onCreated }) {
  const { notify } = useToast()
  const [f, setF] = useState({
    name: '', address: '', roof_area: 0, orientation: '正南',
    load_assessment: '', shading: '', ownership_note: '',
    grid_condition: '', construction_impact: '',
    total_capacity_kw: 0, panel_total: 0, unit_price: 0, maintenance_fund_ratio: 20,
  })
  const set = (k) => (e) => setF({ ...f, [k]: e.target.type === 'number' ? Number(e.target.value) : e.target.value })

  const submit = async () => {
    if (!f.name) return notify('请填写楼栋名称', true)
    try {
      await handleErr(api('POST', '/buildings', f), notify)
      notify('楼栋建档成功')
      onCreated()
    } catch { /* toast */ }
  }

  return (
    <div className="card">
      <h2>新增楼栋建档</h2>
      <p className="muted small">
        记录屋顶面积、朝向、承重评估、遮挡、产权关系、公共收益规则、并网条件与施工影响。
      </p>
      <div className="form-row">
        <label className="field"><span>楼栋名称 *</span><input value={f.name} onChange={set('name')} /></label>
        <label className="field"><span>地址</span><input value={f.address} onChange={set('address')} /></label>
      </div>
      <div className="form-row">
        <label className="field"><span>屋顶面积 m²</span><input type="number" value={f.roof_area} onChange={set('roof_area')} /></label>
        <label className="field"><span>朝向</span><input value={f.orientation} onChange={set('orientation')} /></label>
        <label className="field"><span>装机容量 kW</span><input type="number" value={f.total_capacity_kw} onChange={set('total_capacity_kw')} /></label>
      </div>
      <div className="form-row">
        <label className="field"><span>组件总数（块）</span><input type="number" value={f.panel_total} onChange={set('panel_total')} /></label>
        <label className="field"><span>每块单价（元）</span><input type="number" value={f.unit_price} onChange={set('unit_price')} /></label>
        <label className="field"><span>公共收益计提维修基金比例 %</span><input type="number" value={f.maintenance_fund_ratio} onChange={set('maintenance_fund_ratio')} /></label>
      </div>
      <label className="field"><span>承重评估</span><textarea value={f.load_assessment} onChange={set('load_assessment')} /></label>
      <label className="field"><span>遮挡情况</span><textarea value={f.shading} onChange={set('shading')} /></label>
      <label className="field"><span>产权关系</span><textarea value={f.ownership_note} onChange={set('ownership_note')} /></label>
      <label className="field"><span>并网条件</span><textarea value={f.grid_condition} onChange={set('grid_condition')} /></label>
      <label className="field"><span>施工影响</span><textarea value={f.construction_impact} onChange={set('construction_impact')} /></label>
      <button className="primary" onClick={submit}>保存建档（进入立项阶段）</button>
    </div>
  )
}
