import React, { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { Badge, handleErr, useToast } from '../ui'

export default function VoteTab({ user, building, reload }) {
  const { notify } = useToast()
  const [votes, setVotes] = useState([])
  const [members, setMembers] = useState([])
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const isManager = user.role === 'admin' || user.role === 'property'

  const load = async () => {
    const [v, m] = await Promise.all([
      api('GET', `/buildings/${building.id}/votes`),
      api('GET', `/buildings/${building.id}/members`).catch(() => ({ items: [] })),
    ])
    setVotes(v.items || [])
    setMembers(m.items || [])
  }
  useEffect(() => { load() }, [building.id])

  const create = async () => {
    if (!title) return notify('请填写表决议题', true)
    try {
      await handleErr(api('POST', `/buildings/${building.id}/votes`, { title, content }), notify)
      notify('表决已发起')
      setTitle(''); setContent('')
      load()
    } catch { /* toast */ }
  }

  const choose = async (vid, choice) => {
    try {
      await handleErr(api('POST', `/votes/${vid}/choose`, { choice }), notify)
      load()
    } catch { /* toast */ }
  }

  const close = async (vid) => {
    try {
      const r = await handleErr(api('POST', `/votes/${vid}/close`), notify)
      notify(`表决已结束：${r.result === 'approved' ? '通过（同意≥2/3住户）' : '未通过'}`)
      load()
      if (r.result === 'approved') reload()
    } catch { /* toast */ }
  }

  return (
    <>
      {isManager && (
        <div className="card">
          <h2>发起楼栋立项表决</h2>
          <label className="field"><span>议题 *</span><input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="如：是否同意在本楼屋顶建设分布式光伏" /></label>
          <label className="field"><span>方案说明（可回应漏水担忧、施工影响等）</span><textarea value={content} onChange={(e) => setContent(e.target.value)} /></label>
          <button className="primary" onClick={create}>发起表决</button>
        </div>
      )}

      <div className="card">
        <h2>表决记录</h2>
        <p className="muted small">住户数（有表决权）：{members.filter((m) => m.member_type === 'resident').length} 户 · 通过门槛：同意 ≥ 三分之二</p>
        {votes.length === 0 && <div className="list-empty">暂无表决</div>}
        {votes.map((v) => {
          const total = v.agree + v.disagree + v.abstain
          return (
            <div key={v.id} className="muted-box" style={{ marginBottom: 12 }}>
              <div className="card-title" style={{ marginBottom: 6 }}>
                <h3 style={{ margin: 0 }}>{v.title}</h3>
                <div className="btn-row">
                  {v.status === 'open' ? <Badge value="open" /> : <Badge value={v.result} />}
                </div>
              </div>
              {v.content && <div className="small" style={{ marginBottom: 8 }}>{v.content}</div>}
              <div className="btn-row small" style={{ marginBottom: 8 }}>
                <span className="badge green">同意 {v.agree}</span>
                <span className="badge red">反对 {v.disagree}</span>
                <span className="badge gray">弃权 {v.abstain}</span>
                <span className="muted">已投票 {total} 户</span>
              </div>
              <div className="btn-row">
                {v.status === 'open' && (
                  <>
                    <button className="sm primary" onClick={() => choose(v.id, 'agree')}>投同意票</button>
                    <button className="sm danger" onClick={() => choose(v.id, 'disagree')}>投反对票</button>
                    <button className="sm" onClick={() => choose(v.id, 'abstain')}>弃权</button>
                    {isManager && <button className="sm" onClick={() => close(v.id)}>计票并结束表决</button>}
                  </>
                )}
                {v.status === 'closed' && (
                  <span className="small muted">
                    结论：{v.result === 'approved' ? '✅ 立项通过，可进入认购缴费阶段' : '❌ 未达三分之二，项目需调整方案后重新表决'}
                  </span>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </>
  )
}
