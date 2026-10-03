import { useState } from 'react'
import { get, post } from '../api'
import { useLoader, Btn, Badge, STAGE_TONE, Toast, useToast } from '../ui'
import { STAGES } from '../dict'
import Overview from './tabs/Overview.jsx'
import Votes from './tabs/Votes.jsx'
import Subscriptions from './tabs/Subscriptions.jsx'
import ScheduleDevices from './tabs/ScheduleDevices.jsx'
import Generation from './tabs/Generation.jsx'
import Repairs from './tabs/Repairs.jsx'
import Settlements from './tabs/Settlements.jsx'
import Account from './tabs/Account.jsx'
import Transfers from './tabs/Transfers.jsx'
import Annual from './tabs/Annual.jsx'

const TABS = [
  ['overview', '楼栋概览'],
  ['vote', '表决·公告·异议'],
  ['sub', '认购缴费'],
  ['schedule', '排期与设备'],
  ['gen', '发电计量'],
  ['repair', '维修工单'],
  ['settle', '月度结算'],
  ['account', '公共账户'],
  ['transfer', '房屋转让'],
  ['annual', '年度报告'],
]

export default function BuildingDetail({ user, buildingId, onBack }) {
  const [tab, setTab] = useState('overview')
  const [toast, show] = useToast()
  const { data: b, reload } = useLoader(() => get(`/api/buildings/${buildingId}`), [buildingId])
  const { data: residents, reload: reloadResidents } = useLoader(() => get(`/api/buildings/${buildingId}/residents`), [buildingId])

  const isManager = user.kind === 'ADMIN' || user.role === 'PROPERTY'
  const stageIdx = STAGES.findIndex(([k]) => k === b?.stage)

  const advance = async (action) => {
    try {
      const r = await post(`/api/buildings/${buildingId}/stage`, { action })
      show(`阶段已调整为:${r.stage_name}`)
      reload()
    } catch (e) {
      show(e.message, 'err')
    }
  }

  const ctx = {
    user, buildingId, isManager, residents: residents || [],
    building: b,
    refresh: () => { reload(); reloadResidents() },
    show,
  }

  return (
    <>
      <header className="topbar">
        <div className="row-actions">
          <Btn size="sm" onClick={onBack}>← 返回</Btn>
          <h2>{b?.name || '加载中…'} <Badge tone={STAGE_TONE[b?.stage]}>{b?.stage_name}</Badge></h2>
        </div>
        {isManager && b && (
          <div className="row-actions">
            <Btn size="sm" tone="warn" disabled={stageIdx <= 0} onClick={() => advance('BACK')}>回退阶段</Btn>
            <Btn size="sm" tone="primary" disabled={stageIdx >= STAGES.length - 1}
              title={stageIdx >= STAGES.length - 1 ? '已处于最终阶段' : '校验门槛并进入下一阶段'}
              onClick={() => advance('NEXT')}>
              推进到下一阶段 →
            </Btn>
          </div>
        )}
      </header>
      <div className="content">
        {b && (
          <div className="steps">
            {STAGES.map(([k, name], i) => (
              <span key={k} style={{ display: 'contents' }}>
                <div className={'step' + (i < stageIdx ? ' done' : i === stageIdx ? ' cur' : '')}>
                  <span className="dot">{i < stageIdx ? '✓' : i + 1}</span>
                  {name}
                </div>
                {i < STAGES.length - 1 && <span className={'step-line' + (i < stageIdx ? ' done' : '')} />}
              </span>
            ))}
          </div>
        )}

        <div className="tabs">
          {TABS.map(([k, name]) => (
            <button key={k} className={tab === k ? 'on' : ''} onClick={() => setTab(k)}>{name}</button>
          ))}
        </div>

        {tab === 'overview' && <Overview {...ctx} />}
        {tab === 'vote' && <Votes {...ctx} />}
        {tab === 'sub' && <Subscriptions {...ctx} />}
        {tab === 'schedule' && <ScheduleDevices {...ctx} />}
        {tab === 'gen' && <Generation {...ctx} />}
        {tab === 'repair' && <Repairs {...ctx} />}
        {tab === 'settle' && <Settlements {...ctx} />}
        {tab === 'account' && <Account {...ctx} />}
        {tab === 'transfer' && <Transfers {...ctx} />}
        {tab === 'annual' && <Annual {...ctx} />}
      </div>
      <Toast msg={toast} />
    </>
  )
}
