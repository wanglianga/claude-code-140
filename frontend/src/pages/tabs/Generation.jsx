import { useState } from 'react'
import { get, post } from '../../api'
import { useLoader, Card, Btn, Table, Modal, Field, Bars, NUM } from '../../ui'

export default function Generation({ buildingId, isManager, show }) {
  const { data: meters, reload } = useLoader(() => get(`/api/buildings/${buildingId}/meters`), [buildingId])
  const { data: devices } = useLoader(() => get(`/api/buildings/${buildingId}/devices`), [buildingId])
  const { data: notes, reload: reloadNotes } = useLoader(() => get(`/api/buildings/${buildingId}/generation-notes`), [buildingId])
  const [w, setW] = useState(false)
  const [nw, setNw] = useState(false)

  const totals = Object.entries(meters?.period_totals || {})
    .sort(([a], [b]) => (a < b ? 1 : -1))
    .map(([period, value]) => ({ label: period.slice(5) + '月', value }))
    .reverse()

  return (
    <>
      <div className="grid g2">
        <Card title="月度发电量趋势(kWh)">
          <Bars data={totals} unit="kWh" color="#2f9e6e" format={(v) => NUM(v, 0)} />
        </Card>
        <Card title="发电异常解释" extra={isManager && <Btn size="sm" tone="primary" onClick={() => setNw(true)}>+ 发布解释</Btn>}>
          <div className="timeline">
            {(notes || []).map((n) => (
              <div className="ti" key={n.id}>
                <div className="tt">[{n.period}] {n.title}</div>
                <div className="tm">{n.created_at} · {n.author}</div>
                <div className="td" style={{ whiteSpace: 'pre-wrap' }}>{n.explanation}</div>
              </div>
            ))}
            {(!notes || notes.length === 0) && <div className="muted">暂无异常说明</div>}
          </div>
        </Card>
      </div>

      <Card title="分设备月度读数" extra={isManager && <Btn size="sm" tone="primary" onClick={() => setW(true)}>+ 录入/修改读数</Btn>}>
        <Table head={['周期', '逆变器', '发电量(kWh)', '备注', '录入时间']}>
          {(meters?.readings || []).map((r) => (
            <tr key={r.id}>
              <td>{r.period}</td><td>{r.device_code}</td>
              <td className="num"><b>{NUM(r.reading_kwh, 1)}</b></td>
              <td className="list-text">{r.note || '—'}</td><td>{r.created_at}</td>
            </tr>
          ))}
        </Table>
        <div className="note-box">同一设备同一周期重复录入将覆盖原值;发电量为0或明显偏低的周期,请在右侧"发电异常解释"中向住户说明原因(天气、检修、逆变器故障等)。</div>
      </Card>

      {w && (
        <MeterWrite buildingId={buildingId} devices={devices || []} onClose={() => setW(false)}
          onDone={() => { setW(false); reload(); show('读数已保存') }} />
      )}
      {nw && <NoteWrite buildingId={buildingId} onClose={() => setNw(false)} onDone={() => { setNw(false); reloadNotes(); show('异常解释已发布') }} />}
    </>
  )
}

function MeterWrite({ buildingId, devices, onClose, onDone }) {
  const [f, setF] = useState({ device_id: devices[0]?.id || 0, period: '2026-01', reading_kwh: '', note: '' })
  const set = (k) => (e) => setF({ ...f, [k]: e.target.value })
  return (
    <Modal title="录入月度发电量" onClose={onClose}>
      <Field label="逆变器">
        <select value={f.device_id} onChange={set('device_id')}>
          <option value={0}>请选择</option>
          {devices.map((d) => <option key={d.id} value={d.id}>{d.code} {d.name}(#{d.component_from}~#{d.component_to})</option>)}
        </select>
      </Field>
      <div className="form-row">
        <Field label="结算周期"><input type="month" value={f.period} onChange={set('period')} /></Field>
        <Field label="发电量(kWh)"><input type="number" min="0" value={f.reading_kwh} onChange={set('reading_kwh')} placeholder="如 3120.5" /></Field>
      </div>
      <Field label="备注(如设备检修/故障停机)"><input value={f.note} onChange={set('note')} /></Field>
      <div className="modal-ft"><Btn onClick={onClose}>取消</Btn>
        <Btn tone="primary" onClick={async () => {
          try {
            await post(`/api/buildings/${buildingId}/meters`, {
              ...f, device_id: Number(f.device_id), reading_kwh: Number(f.reading_kwh),
            })
            onDone()
          } catch (e) { alert(e.message) }
        }}>保存读数</Btn></div>
    </Modal>
  )
}

function NoteWrite({ buildingId, onClose, onDone }) {
  const [f, setF] = useState({ period: '2026-01', title: '', explanation: '' })
  const set = (k) => (e) => setF({ ...f, [k]: e.target.value })
  return (
    <Modal title="发电异常解释" onClose={onClose}>
      <div className="form-row">
        <Field label="周期"><input type="month" value={f.period} onChange={set('period')} /></Field>
        <Field label="标题"><input value={f.title} onChange={set('title')} placeholder="如:2月发电量偏低说明" /></Field>
      </div>
      <Field label="解释内容">
        <textarea style={{ minHeight: 130 }} value={f.explanation} onChange={set('explanation')}
          placeholder="说明原因(连续阴雨/逆变器故障已维修/组件清洗/电网限电…)、影响电量与后续措施" />
      </Field>
      <div className="modal-ft"><Btn onClick={onClose}>取消</Btn>
        <Btn tone="primary" onClick={async () => {
          try { await post(`/api/buildings/${buildingId}/generation-notes`, f); onDone() }
          catch (e) { alert(e.message) }
        }}>发布</Btn></div>
    </Modal>
  )
}
