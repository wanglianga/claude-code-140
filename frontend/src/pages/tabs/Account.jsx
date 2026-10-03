import { get } from '../../api'
import { useLoader, Card, Badge, Table, CNY } from '../../ui'
import { TX_TYPE } from '../../dict'

export default function Account({ buildingId, building: b }) {
  const { data } = useLoader(() => get(`/api/buildings/${buildingId}/public-transactions`), [buildingId])
  return (
    <>
      <div className="grid g3">
        <div className="stat">
          <div className="k">公共账户当前余额</div>
          <div className="v green">¥{CNY(data?.balance)}</div>
          <div className="sub">发电结算、补贴、赔偿入账;维修、运维、税费、退款、收益发放出账</div>
        </div>
        <div className="stat">
          <div className="k">累计计提维修基金</div>
          <div className="v">¥{CNY(data?.repair_fund_total)}</div>
          <div className="sub">按每月毛收益 {b.repair_fund_pct}% 计提(已确认结算)</div>
        </div>
        <div className="stat">
          <div className="k">公共收益规则</div>
          <div style={{ fontSize: 12.5, marginTop: 8, color: '#475569', lineHeight: 1.7 }}>{b.public_benefit_rule}</div>
        </div>
      </div>

      <Card title="公共账户流水(最近300笔)">
        <Table head={['时间', '周期', '类型', '方向', '金额', '余额', '事由']}>
          {(data?.transactions || []).map((t) => (
            <tr key={t.id}>
              <td>{t.created_at}</td>
              <td>{t.period || '—'}</td>
              <td>{TX_TYPE[t.type] || t.type}</td>
              <td>{t.direction === 'IN' ? <Badge tone="green">入账</Badge> : <Badge tone="red">出账</Badge>}</td>
              <td className="num" style={{ color: t.direction === 'IN' ? 'var(--green-d)' : 'var(--red)', fontWeight: 700 }}>
                {t.direction === 'IN' ? '+' : '-'}¥{CNY(t.amount)}
              </td>
              <td className="num">¥{CNY(t.balance_after)}</td>
              <td className="list-text">{t.remark}</td>
            </tr>
          ))}
        </Table>
      </Card>
    </>
  )
}
