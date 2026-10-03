import { useState } from 'react'
import { get, post } from '../../api'
import { useLoader, Card, Btn, Badge, Table, Modal, Field, Btn as Button } from '../../ui'
import { VOTE_LABEL, VOTE_TONE, OBJECTION_STATUS, OBJECTION_TONE } from '../../dict'

export default function Votes({ user, buildingId, residents, isManager, show }) {
  const [pane, setPane] = useState('vote')
  return (
    <>
      <div className="tabs">
        <button className={pane === 'vote' ? 'on' : ''} onClick={() => setPane('vote')}>楼栋立项表决</button>
        <button className={pane === 'ann' ? 'on' : ''} onClick={() => setPane('ann')}>楼栋公告</button>
        <button className={pane === 'obj' ? 'on' : ''} onClick={() => setPane('obj')}>住户异议与投票</button>
      </div>
      {pane === 'vote' && <VotePane user={user} buildingId={buildingId} isManager={isManager} show={show} />}
      {pane === 'ann' && <AnnPane buildingId={buildingId} isManager={isManager} show={show} />}
      {pane === 'obj' && <ObjPane user={user} buildingId={buildingId} isManager={isManager} show={show} />}
    </>
  )
}

function VotePane({ user, buildingId, isManager, show }) {
  const { data, reload } = useLoader(() => get(`/api/buildings/${buildingId}/votes`), [buildingId])
  const [vote, setVote] = useState('APPROVE')
  const [opinion, setOpinion] = useState('')
  const t = data?.tally
  const submit = async () => {
    try {
      await post(`/api/buildings/${buildingId}/votes`, { vote, opinion })
      show('投票成功')
      reload()
    } catch (e) { show(e.message, 'err') }
  }
  return (
    <>
      <div className="grid g3">
        <Card title="表决进度(户数 / 面积双门槛)">
          <div style={{ display: 'grid', gap: 12 }}>
            <ProgressBar label="户数赞成率" pct={t?.household_pct || 0} threshold={t?.threshold_pct || 0}
              text={`${t?.approve ?? 0} / ${t?.total ?? 0} 户 · ${t?.household_pct ?? 0}%`} />
            <ProgressBar label="面积赞成率" pct={t?.area_pct || 0} threshold={t?.threshold_pct || 0}
              text={`${t?.area_pct ?? 0}% 产权面积赞成`} />
            <div className="row-actions" style={{ gap: 18, fontSize: 12.5, color: '#475569' }}>
              <span>赞成 <b style={{ color: 'var(--green-d)' }}>{t?.approve ?? 0}</b></span>
              <span>反对 <b style={{ color: 'var(--red)' }}>{t?.reject ?? 0}</b></span>
              <span>弃权 <b>{t?.abstain ?? 0}</b></span>
              <span>门槛 {t?.threshold_pct ?? 0}%</span>
            </div>
            {t?.passed ? <Badge tone="green">✓ 已达到通过门槛,可推进至认购缴费</Badge> : <Badge tone="amber">尚未达到通过门槛</Badge>}
          </div>
        </Card>

        <Card title="我的表决意见">
          {user.kind === 'RESIDENT' ? (
            <>
              <Field label="我的投票">
                <select value={vote} onChange={(e) => setVote(e.target.value)}>
                  <option value="APPROVE">赞成</option>
                  <option value="REJECT">反对</option>
                  <option value="ABSTAIN">弃权</option>
                </select>
              </Field>
              <div style={{ height: 10 }} />
              <Field label="补充意见(如顶楼漏水担忧、通道维护要求等)">
                <textarea value={opinion} onChange={(e) => setOpinion(e.target.value)}
                  placeholder="例:担心施工破坏防水层,要求出具防水保修承诺" />
              </Field>
              <div style={{ height: 12 }} />
              <Btn tone="primary" onClick={submit}>提交 / 修改我的表决</Btn>
            </>
          ) : <div className="muted">管理员不参与住户表决,可在此查看计票结果。</div>}
        </Card>

        <Card title="计票规则">
          <div className="list-text">
            依据《民法典》业主共同决定事项,本项目须经参与表决专有部分面积过半数与参与表决人数过半数同意方可推进。
            {'\n'}平台以本楼登记产权面积与户数分别统计,两条均达到楼栋设定门槛({t?.threshold_pct ?? 67}%)才允许进入认购缴费阶段。
            {'\n'}住户可反复修改本人投票,以最后一次为准。
          </div>
        </Card>
      </div>

      <Card title="表决明细">
        <Table head={['房号', '业主', '产权面积', '投票', '意见', '时间']}>
          {(data?.votes || []).map((v) => (
            <tr key={v.resident_id}>
              <td>{v.unit_no}</td><td>{v.owner_name}</td><td className="num">{v.area_owned} m²</td>
              <td><Badge tone={VOTE_TONE[v.vote]}>{VOTE_LABEL[v.vote]}</Badge></td>
              <td className="list-text">{v.opinion || '—'}</td><td>{v.created_at}</td>
            </tr>
          ))}
        </Table>
      </Card>
    </>
  )
}

function ProgressBar({ label, pct, threshold, text }) {
  const pass = pct >= threshold
  return (
    <div>
      <div className="row-actions" style={{ justifyContent: 'space-between', fontSize: 12.5, marginBottom: 5 }}>
        <span>{label}</span><span>{text}</span>
      </div>
      <div className="bar-pct" style={{ width: '100%', position: 'relative' }}>
        <i style={{ width: Math.min(100, pct) + '%', background: pass ? 'var(--green)' : 'var(--amber)' }} />
        <i style={{ position: 'absolute', top: -2, bottom: -2, width: 2, background: '#334155', left: Math.min(100, threshold) + '%' }} />
      </div>
    </div>
  )
}

function AnnPane({ buildingId, isManager, show }) {
  const { data, reload } = useLoader(() => get(`/api/buildings/${buildingId}/announcements`), [buildingId])
  const [w, setW] = useState(false)
  return (
    <>
      <Card title="楼栋公告" extra={isManager && <Btn size="sm" tone="primary" onClick={() => setW(true)}>+ 发布公告</Btn>}>
        <div className="timeline">
          {(data || []).map((a) => (
            <div className="ti" key={a.id}>
              <div className="tt">{a.title}</div>
              <div className="tm">{a.created_at} · {a.author}</div>
              <div className="td" style={{ whiteSpace: 'pre-wrap' }}>{a.content}</div>
            </div>
          ))}
          {(!data || data.length === 0) && <div className="muted">暂无公告</div>}
        </div>
      </Card>
      {w && <AnnWrite buildingId={buildingId} onClose={() => setW(false)} onDone={() => { setW(false); reload(); show('公告已发布') }} />}
    </>
  )
}

function AnnWrite({ buildingId, onClose, onDone }) {
  const [f, setF] = useState({ title: '', content: '' })
  return (
    <Modal title="发布楼栋公告" onClose={onClose}>
      <Field label="标题"><input value={f.title} onChange={(e) => setF({ ...f, title: e.target.value })} /></Field>
      <Field label="内容"><textarea style={{ minHeight: 130 }} value={f.content} onChange={(e) => setF({ ...f, content: e.target.value })} /></Field>
      <div className="modal-ft"><Btn onClick={onClose}>取消</Btn>
        <Btn tone="primary" onClick={async () => { try { await post(`/api/buildings/${buildingId}/announcements`, f); onDone() } catch (e) { alert(e.message) } }}>发布</Btn></div>
    </Modal>
  )
}

function ObjPane({ user, buildingId, isManager, show }) {
  const { data, reload } = useLoader(() => get(`/api/buildings/${buildingId}/objections`), [buildingId])
  const [w, setW] = useState(false)
  const [voteOpen, setVoteOpen] = useState(null)
  const [f, setF] = useState({ title: '', content: '' })

  const startVote = async (id) => {
    try { await post(`/api/objections/${id}/vote-start`, {}); show('已发起住户投票'); reload() }
    catch (e) { show(e.message, 'err') }
  }
  const closeVote = async (id) => {
    try { const r = await post(`/api/objections/${id}/close`, {}); show('裁决完成:异议' + (r.result === 'APPROVED' ? '成立' : '驳回')); reload() }
    catch (e) { show(e.message, 'err') }
  }

  return (
    <>
      <Card title="住户异议" extra={user.kind === 'RESIDENT' && <Btn size="sm" tone="primary" onClick={() => { setF({ title: '', content: '' }); setW(true) }}>+ 提出异议</Btn>}>
        <Table head={['编号', '提出人', '标题', '内容', '状态', '投票(赞/反/弃)', '操作']}>
          {(data || []).map((o, i) => (
            <tr key={o.id}>
              <td>#{i + 1}</td>
              <td>{o.unit_no} {o.owner_name}</td>
              <td>{o.title}</td>
              <td className="list-text">{o.content}</td>
              <td><Badge tone={OBJECTION_TONE[o.status]}>{OBJECTION_STATUS[o.status]}</Badge></td>
              <td>{o.votes.approve}/{o.votes.reject}/{o.votes.abstain} <span className="muted">(参与率 {o.votes.participation}%)</span></td>
              <td>
                {isManager && o.status === 'OPEN' && <Button size="sm" onClick={() => startVote(o.id)}>发起投票</Button>}
                {user.kind === 'RESIDENT' && o.status === 'VOTING' && <Button size="sm" tone="primary" onClick={() => setVoteOpen(o.id)}>我要投票</Button>}
                {isManager && o.status === 'VOTING' && <Button size="sm" tone="warn" onClick={() => closeVote(o.id)}>裁决关闭</Button>}
              </td>
            </tr>
          ))}
        </Table>
        <div className="note-box">裁决规则:全体在住住户过半数投赞成票则异议成立,物业/平台需据此调整方案(如防水保修、检修通道安排等),相关讨论可在公告中公示。</div>
      </Card>

      {w && (
        <Modal title="提出异议" onClose={() => setW(false)}>
          <Field label="标题"><input value={f.title} onChange={(e) => setF({ ...f, title: e.target.value })}
            placeholder="如:对6月收益测算口径有疑问 / 要求保留楼顶检修通道" /></Field>
          <Field label="详细说明"><textarea style={{ minHeight: 120 }} value={f.content} onChange={(e) => setF({ ...f, content: e.target.value })} /></Field>
          <div className="modal-ft"><Btn onClick={() => setW(false)}>取消</Btn>
            <Btn tone="primary" onClick={async () => {
              try { await post(`/api/buildings/${buildingId}/objections`, f); setW(false); reload(); show('异议已提交,等待受理') }
              catch (e) { show(e.message, 'err') }
            }}>提交异议</Btn></div>
        </Modal>
      )}
      {voteOpen && <ObjectionVote oid={voteOpen} onClose={() => setVoteOpen(null)} onDone={() => { setVoteOpen(null); reload(); show('投票成功') }} />}
    </>
  )
}

function ObjectionVote({ oid, onClose, onDone }) {
  const [v, setV] = useState('APPROVE')
  return (
    <Modal title="异议投票" onClose={onClose}>
      <div className="list-text" style={{ marginBottom: 8 }}>赞成 = 认可异议成立,要求调整方案;反对 = 维持原方案。</div>
      <Field label="我的意见">
        <select value={v} onChange={(e) => setV(e.target.value)}>
          <option value="APPROVE">赞成(异议成立)</option>
          <option value="REJECT">反对(维持原案)</option>
          <option value="ABSTAIN">弃权</option>
        </select>
      </Field>
      <div className="modal-ft"><Btn onClick={onClose}>取消</Btn>
        <Btn tone="primary" onClick={async () => { try { await post(`/api/objections/${oid}/vote`, { vote: v }); onDone() } catch (e) { alert(e.message) } }}>确认投票</Btn></div>
    </Modal>
  )
}
