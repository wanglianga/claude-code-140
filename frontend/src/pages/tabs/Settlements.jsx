import { useState } from 'react'
import { get, post } from '../../api'
import { useLoader, Card, Btn, Badge, Table, Modal, Field, CNY } from '../../ui'
import { SETTLE_STATUS, SETTLE_TONE } from '../../dict'

export default function Settlements({ buildingId, isManager, show }) {
  const { data, reload } = useLoader(() => get(`/api/buildings/${buildingId}/settlements`), [buildingId])
  const [gen, setGen] = useState(false)
  const [period, setPeriod] = useState('2026-01')
  const [detail, setDetail] = useState(null)

  const generate = async () => {
    try {
      const r = await post(`/api/buildings/${buildingId}/settlements`, { period })
      show(`${period} 结算草稿已生成`)
      setGen(false)
      reload()
    } catch (e) { show(e.message, 'err') }
  }
  const confirm = async (id) => {
    if (!confirm('确认后该周期收益将正式入账:住户收益发放、补贴入公共账户、运维税费出账。确认继续?')) return
    try {
      const r = await post(`/api/settlements/${id}/confirm`, {})
      show(`结算已确认入账,住户发放 ¥${CNY(r.resident_payout)}`)
      reload()
    } catch (e) { show(e.message, 'err') }
  }

  return (
    <>
      <Card title="月度结算单" extra={isManager && (
        <div className="row-actions">
          <input type="month" value={period} onChange={(e) => setPeriod(e.target.value)} style={{ width: 150 }} />
          <Btn size="sm" tone="primary" onClick={() => setGen(true)}>生成/重算草稿</Btn>
        </div>
      )}>
        <Table head={['周期', '发电量(kWh)', '发电收入', '补贴', '运维费', '税费', '维修基金', '维修支出', '责任赔偿', '可分净额', '公共收益', '状态', '操作']}>
          {(data || []).map((s) => (
            <tr key={s.id}>
              <td style={{ fontWeight: 600 }}>{s.period}</td>
              <td className="num">{s.total_kwh}</td>
              <td className="num">{CNY(s.gross_income)}</td>
              <td className="num">{CNY(s.subsidy)}</td>
              <td className="num">-{CNY(s.maintenance_fee)}</td>
              <td className="num">-{CNY(s.tax)}</td>
              <td className="num">-{CNY(s.repair_fund)}</td>
              <td className="num">{s.repair_cost ? '-' + CNY(s.repair_cost) : '—'}</td>
              <td className="num" style={{ color: s.deduction ? 'var(--green-d)' : undefined }}>{s.deduction ? '+' + CNY(s.deduction) : '—'}</td>
              <td className="num"><b>{CNY(s.net_income)}</b></td>
              <td className="num">{CNY(s.public_income)}</td>
              <td><Badge tone={SETTLE_TONE[s.status]}>{SETTLE_STATUS[s.status]}</Badge></td>
              <td>
                <div className="row-actions">
                  <Btn size="sm" onClick={() => setDetail(s)}>明细</Btn>
                  {isManager && s.status === 'DRAFT' && <Btn size="sm" tone="primary" onClick={() => confirm(s.id)}>确认入账</Btn>}
                </div>
              </td>
            </tr>
          ))}
        </Table>
        <div className="note-box">
          计算口径:发电收入 = 实际发电量 × 电价;补贴 = 发电量 × 度电补贴;
          可分净额 = 收入 + 补贴 + 责任赔偿 − 运维费 − 税费 − 维修基金计提。
          净额按「已缴费认购组件比例」分配给认购住户(住户部分 = 净额 ×(1−公共比例)),其余进入公共账户。
          已确认的结算不可重算,保证账目不翻烧饼。
        </div>
      </Card>

      {gen && (
        <Modal title={`生成 ${period} 结算草稿`} onClose={() => setGen(false)}>
          <div className="list-text">将汇总 {period} 全部逆变器读数与该周期已处理维修工单,生成草稿(可反复重算,确认前不入账)。</div>
          <div className="modal-ft"><Btn onClick={() => setGen(false)}>取消</Btn><Btn tone="primary" onClick={generate}>生成草稿</Btn></div>
        </Modal>
      )}
      {detail && <SettleDetail s={detail} onClose={() => setDetail(null)} />}
    </>
  )
}

function SettleDetail({ s, onClose }) {
  return (
    <Modal title={`${s.period} 结算明细`} onClose={onClose} wide>
      <div className="grid g3" style={{ marginBottom: 12 }}>
        <Mini k="实际发电量" v={s.total_kwh + ' kWh'} />
        <Mini k="可分配净额" v={'¥' + CNY(s.net_income)} green />
        <Mini k="其中公共收益" v={'¥' + CNY(s.public_income)} />
      </div>
      <Table head={['对象', '类型', '组件占比', '分配电量(kWh)', '分配金额(元)']}>
        {(s.items || []).map((it, i) => (
          <tr key={i}>
            <td>{it.name}</td>
            <td>{it.kind === 'PUBLIC' ? <Badge tone="blue">公共账户</Badge> : <Badge tone="green">认购住户</Badge>}</td>
            <td className="num">{it.kind === 'PUBLIC' ? '—' : it.share_pct + '%'}</td>
            <td className="num">{it.allocated_kwh}</td>
            <td className="num"><b>¥{CNY(it.amount)}</b></td>
          </tr>
        ))}
      </Table>
      {s.note && <div className="note-box">备注:{s.note}</div>}
      <div className="modal-ft"><Btn onClick={onClose}>关闭</Btn></div>
    </Modal>
  )
}

function Mini({ k, v, green }) {
  return (
    <div className="stat">
      <div className="k">{k}</div>
      <div className={'v' + (green ? ' green' : '')} style={{ fontSize: 19 }}>{v}</div>
    </div>
  )
}
