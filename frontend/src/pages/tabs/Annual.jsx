import { useState } from 'react'
import { get } from '../../api'
import { useLoader, Card, Table, Bars, CNY, NUM } from '../../ui'

export default function Annual({ buildingId }) {
  const [year, setYear] = useState('2026')
  const { data } = useLoader(() => get(`/api/buildings/${buildingId}/annual-report?year=${year}`), [buildingId, year])
  const s = data?.summary
  const monthKwh = (data?.months || []).map((m) => ({ label: m.period.slice(5) + '月', value: m.kwh, dim: m.status !== 'CONFIRMED' }))
  const monthNet = (data?.months || []).map((m) => ({ label: m.period.slice(5) + '月', value: m.net, dim: m.status !== 'CONFIRMED' }))

  return (
    <>
      <Card title="年度收益报告" extra={
        <select value={year} onChange={(e) => setYear(e.target.value)} style={{ width: 120 }}>
          <option value="2026">2026 年</option>
          <option value="2025">2025 年</option>
        </select>
      }>
        <div className="grid g4">
          <Stat k="年度发电量" v={NUM(s?.total_kwh, 0)} u="kWh" />
          <Stat k="发电收入(含补贴)" v={'¥' + CNY((s?.gross_income || 0) + (s?.subsidy || 0))} green />
          <Stat k="年度可分净额" v={'¥' + CNY(s?.net_income)} green />
          <Stat k="其中公共收益" v={'¥' + CNY(s?.public_income)} />
          <Stat k="运维费" v={'¥' + CNY(s?.maintenance)} />
          <Stat k="税费" v={'¥' + CNY(s?.tax)} />
          <Stat k="计提维修基金" v={'¥' + CNY(s?.repair_fund)} />
          <Stat k="公共账户总支出" v={'¥' + CNY(s?.public_out)} />
        </div>
      </Card>

      <div className="grid g2">
        <Card title="月度发电量(kWh,浅色为未确认)"><Bars data={monthKwh} unit="kWh" format={(v) => NUM(v, 0)} /></Card>
        <Card title="月度可分配净额(元,浅色为未确认)"><Bars data={monthNet} color="#3b7dd8" unit="元" format={(v) => NUM(v, 0)} /></Card>
      </div>

      <div className="grid g2">
        <div className="carbon">
          <div style={{ opacity: .9, fontSize: 13 }}>年度碳减排(华东电网因子 0.581 kgCO₂/kWh)</div>
          <div className="v">{NUM(s?.carbon_t, 2)} <small>tCO₂</small></div>
          <div style={{ marginTop: 12, display: 'grid', gap: 6, fontSize: 13 }}>
            <div>折合减排二氧化碳 <b>{NUM(s?.carbon_kg, 0)} kg</b></div>
            <div>等效节约标准煤 <b>{NUM(s?.coal_kg, 0)} kg</b></div>
            <div>等效成年树木一年吸碳量 <b>{NUM(s?.trees, 0)} 棵</b></div>
          </div>
        </div>

        <Card title={`维修记录(年度 ${s?.repair_orders ?? 0} 单,已解决 ${s?.repair_resolved ?? 0} 单)`}>
          <div style={{ fontSize: 13.5, color: '#475569', lineHeight: 2.1 }}>
            年度维修支出(公共/物业承担):<b>¥{CNY(s?.repair_cost)}</b><br />
            责任方赔偿计入结算:<b style={{ color: 'var(--green-d)' }}>¥{CNY(s?.deduction)}</b><br />
            详细工单与责任判定见"维修工单"页签。
          </div>
        </Card>
      </div>

      <Card title="住户年度收益榜(已确认结算)">
        <Table head={['排名', '房号', '业主', '归属电量(kWh)', '年度收益(元)']}>
          {(data?.resident_rank || []).map((r, i) => (
            <tr key={i}>
              <td>{i < 3 ? ['🥇', '🥈', '🥉'][i] : i + 1}</td>
              <td>{r.unit_no}</td><td>{r.owner_name}</td>
              <td className="num">{NUM(r.kwh, 1)}</td>
              <td className="num"><b>¥{CNY(r.amount)}</b></td>
            </tr>
          ))}
          {(!data?.resident_rank || data.resident_rank.length === 0) && (
            <tr><td colSpan="5" className="empty">{year} 年暂无已确认结算</td></tr>
          )}
        </Table>
      </Card>
    </>
  )
}

function Stat({ k, v, u, green }) {
  return (
    <div className="stat">
      <div className="k">{k}</div>
      <div className={'v' + (green ? ' green' : '')} style={{ fontSize: 20 }}>{v} {u && <small>{u}</small>}</div>
    </div>
  )
}
