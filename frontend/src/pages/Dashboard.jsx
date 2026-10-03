import { useState } from 'react'
import { get, post } from '../api'
import { useLoader, Card, Btn, Badge, STAGE_TONE, Modal, Field, Toast, useToast, CNY, NUM, Donut } from '../ui'

export default function Dashboard({ user, onOpen }) {
  const { data: stats, reload: reloadStats } = useLoader(() => get('/api/dashboard'), [])
  const { data: buildings, reload } = useLoader(() => get('/api/buildings'), [])
  const [showCreate, setShowCreate] = useState(false)
  const [toast, show] = useToast()
  const isAdmin = user.kind === 'ADMIN'

  return (
    <>
      <header className="topbar">
        <h2>{isAdmin ? '社区光伏楼栋总览' : '我的楼栋'}</h2>
        {isAdmin && <Btn tone="primary" onClick={() => setShowCreate(true)}>+ 引入光伏楼栋</Btn>}
      </header>
      <div className="content">
        <div className="grid g4">
          <Stat k="光伏楼栋" v={stats?.building_count ?? '-'} u="栋" />
          <Stat k="总装机容量" v={NUM(stats?.total_capacity, 1)} u="kWp" />
          <Stat k="累计发电量" v={NUM(stats?.total_kwh, 0)} u="kWh" />
          <Stat k="累计可分配收益" v={'¥' + CNY(stats?.total_income)} green />
          <Stat k="公共账户余额" v={'¥' + CNY(stats?.public_balance)} />
          <Stat k="逆变器/设备" v={stats?.device_count ?? '-'} u="台" />
          <Stat k="处理中维修单" v={stats?.open_repairs ?? '-'} u="单" tone={stats?.open_repairs ? 'red' : ''} />
          <Stat k="累计碳减排" v={NUM(stats?.carbon_t, 2)} u={`tCO₂ ≈ ${NUM(stats?.carbon_kg, 0)} kg`} green />
        </div>

        <Card title="楼栋列表" extra={<Btn size="sm" onClick={() => { reload(); reloadStats() }}>刷新</Btn>}>
          <div className="grid g2">
            {(buildings || []).map((b) => (
              <div key={b.id} className="card" style={{ marginBottom: 0 }}>
                <div className="card-bd">
                  <div className="row-actions" style={{ justifyContent: 'space-between' }}>
                    <div>
                      <h3 style={{ fontSize: 16 }}>{b.name} <span className="muted" style={{ fontSize: 12 }}>({b.code})</span></h3>
                      <div className="muted" style={{ fontSize: 12, marginTop: 3 }}>{b.address} · {b.resident_count} 户 · {NUM(b.total_capacity_kw, 1)} kWp</div>
                    </div>
                    <Badge tone={STAGE_TONE[b.stage]}>{b.stage_name}</Badge>
                  </div>
                  <div className="row-actions" style={{ marginTop: 14, justifyContent: 'space-between' }}>
                    <Donut pct={b.subscribe_pct} size={96} label="组件认购" />
                    <div style={{ flex: 1, paddingLeft: 18, fontSize: 12.5, color: '#475569', display: 'grid', gap: 5 }}>
                      <div>组件认购:<b>{b.paid_components}</b> / {b.component_count} 块 · 公共剩余 {b.component_count - b.paid_components} 块</div>
                      <div>累计发电:<b>{NUM(b.total_kwh, 0)} kWh</b></div>
                      <div>公共账户:<b>¥{CNY(b.public_balance)}</b></div>
                      <div style={{ marginTop: 4 }}>
                        <Btn tone="primary" size="sm" onClick={() => onOpen(b.id)}>进入楼栋工作台 →</Btn>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            ))}
            {buildings && buildings.length === 0 && <div className="muted">暂无楼栋</div>}
          </div>
        </Card>
      </div>

      {showCreate && <CreateBuilding onClose={() => setShowCreate(false)} onDone={() => { setShowCreate(false); reload(); reloadStats(); show('楼栋已创建') }} />}
      <Toast msg={toast} />
    </>
  )
}

function Stat({ k, v, u, green, tone }) {
  return (
    <div className="stat">
      <div className="k">{k}</div>
      <div className={'v' + (green ? ' green' : '') + (tone === 'red' ? '' : '')} style={tone === 'red' ? { color: 'var(--red)' } : undefined}>
        {v} {u && <small>{u}</small>}
      </div>
    </div>
  )
}

function CreateBuilding({ onClose, onDone }) {
  const [f, setF] = useState({
    code: '', name: '', address: '', rooftop_area: 400, orientation: '正南',
    load_assessment: '', shading: '', property_rights: '', public_benefit_rule: '公共部分收益20%计入楼栋公共账户,年底公示',
    grid_condition: '', construction_impact: '',
    total_capacity_kw: 100, component_count: 200, component_unit_price: 1200,
    public_share_pct: 20, vote_threshold_pct: 67, electricity_price: 0.39,
    subsidy_rate: 0.05, maintenance_fee_pct: 5, tax_pct: 3, repair_fund_pct: 3,
  })
  const set = (k) => (e) => setF({ ...f, [k]: e.target.value })
  const submit = async () => {
    const numF = { ...f }
    ;['rooftop_area','total_capacity_kw','component_unit_price','public_share_pct','vote_threshold_pct',
      'electricity_price','subsidy_rate','maintenance_fee_pct','tax_pct','repair_fund_pct']
      .forEach((k) => (numF[k] = Number(numF[k]) || 0))
    numF.component_count = Number(numF.component_count) || 0
    await post('/api/buildings', numF)
    onDone()
  }
  return (
    <Modal title="引入光伏楼栋(立项评估信息)" onClose={onClose} wide>
      <div className="form-row">
        <Field label="楼栋编号"><input value={f.code} onChange={set('code')} placeholder="B-002" /></Field>
        <Field label="楼栋名称"><input value={f.name} onChange={set('name')} /></Field>
      </div>
      <Field label="地址"><input value={f.address} onChange={set('address')} /></Field>
      <div className="form-row">
        <Field label="屋顶面积(m²)"><input type="number" value={f.rooftop_area} onChange={set('rooftop_area')} /></Field>
        <Field label="朝向"><input value={f.orientation} onChange={set('orientation')} /></Field>
      </div>
      <Field label="承重评估"><textarea value={f.load_assessment} onChange={set('load_assessment')} /></Field>
      <div className="form-row">
        <Field label="遮挡情况"><input value={f.shading} onChange={set('shading')} /></Field>
        <Field label="产权关系"><input value={f.property_rights} onChange={set('property_rights')} /></Field>
      </div>
      <Field label="公共收益规则"><input value={f.public_benefit_rule} onChange={set('public_benefit_rule')} /></Field>
      <div className="form-row">
        <Field label="并网条件"><input value={f.grid_condition} onChange={set('grid_condition')} /></Field>
        <Field label="施工影响"><input value={f.construction_impact} onChange={set('construction_impact')} /></Field>
      </div>
      <div className="form-row">
        <Field label="装机容量(kWp)"><input type="number" value={f.total_capacity_kw} onChange={set('total_capacity_kw')} /></Field>
        <Field label="组件总数(块)"><input type="number" value={f.component_count} onChange={set('component_count')} /></Field>
      </div>
      <div className="form-row">
        <Field label="单组件价格(元)"><input type="number" value={f.component_unit_price} onChange={set('component_unit_price')} /></Field>
        <Field label="公共部分比例(%)"><input type="number" value={f.public_share_pct} onChange={set('public_share_pct')} /></Field>
      </div>
      <div className="form-row">
        <Field label="表决门槛(%)"><input type="number" value={f.vote_threshold_pct} onChange={set('vote_threshold_pct')} /></Field>
        <Field label="结算电价(元/kWh)"><input type="number" step="0.01" value={f.electricity_price} onChange={set('electricity_price')} /></Field>
      </div>
      <div className="form-row">
        <Field label="度电补贴(元/kWh)"><input type="number" step="0.01" value={f.subsidy_rate} onChange={set('subsidy_rate')} /></Field>
        <Field label="运维费比例(%)"><input type="number" value={f.maintenance_fee_pct} onChange={set('maintenance_fee_pct')} /></Field>
      </div>
      <div className="form-row">
        <Field label="税费比例(%)"><input type="number" value={f.tax_pct} onChange={set('tax_pct')} /></Field>
        <Field label="维修基金计提(%)"><input type="number" value={f.repair_fund_pct} onChange={set('repair_fund_pct')} /></Field>
      </div>
      <div className="modal-ft">
        <Btn onClick={onClose}>取消</Btn>
        <Btn tone="primary" onClick={submit}>创建</Btn>
      </div>
    </Modal>
  )
}
