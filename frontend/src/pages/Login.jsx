import { useState } from 'react'
import { post, saveAuth } from '../api'
import { Btn, Field } from '../ui'

export default function Login({ onLogin }) {
  const [as, setAs] = useState('ADMIN')
  const [identifier, setIdentifier] = useState('admin')
  const [password, setPassword] = useState('admin123')
  const [buildingId, setBuildingId] = useState('')
  const [err, setErr] = useState('')
  const [loading, setLoading] = useState(false)

  const switchAs = (k) => {
    setAs(k)
    setErr('')
    if (k === 'ADMIN') {
      setIdentifier('admin')
      setPassword('admin123')
    } else {
      setIdentifier('钱进')
      setPassword('123456')
    }
  }

  const submit = async (e) => {
    e.preventDefault()
    setErr('')
    setLoading(true)
    try {
      const res = await post('/api/login', {
        as,
        identifier: identifier.trim(),
        password,
        building_id: buildingId ? Number(buildingId) : 0,
      })
      saveAuth(res.token, res.user)
      onLogin(res.user)
    } catch (e) {
      setErr(e.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-wrap">
      <form className="login" onSubmit={submit}>
        <div className="login-hd">
          <div className="logo">☀️ 屋顶光伏认购结算平台</div>
          <p>居民楼分布式光伏 · 认购 / 发电 / 结算 / 维修 全周期账目</p>
        </div>
        <div className="login-bd">
          <div className="seg">
            <button type="button" className={as === 'ADMIN' ? 'on' : ''} onClick={() => switchAs('ADMIN')}>
              平台管理员
            </button>
            <button type="button" className={as === 'RESIDENT' ? 'on' : ''} onClick={() => switchAs('RESIDENT')}>
              住户 / 物业
            </button>
          </div>
          <Field label={as === 'ADMIN' ? '管理员账号' : '住户姓名 或 房号'}>
            <input value={identifier} onChange={(e) => setIdentifier(e.target.value)} placeholder={as === 'ADMIN' ? 'admin' : '如:钱进 或 601'} />
          </Field>
          {as === 'RESIDENT' && (
            <Field label="楼栋ID(多楼栋时填写,演示数据为 1)" hint="单栋演示可留空">
              <input value={buildingId} onChange={(e) => setBuildingId(e.target.value)} placeholder="1" />
            </Field>
          )}
          <Field label="密码">
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
          </Field>
          {err && <div className="toast toast-err" style={{ position: 'static', transform: 'none' }}>{err}</div>}
          <Btn type="submit" tone="primary" disabled={loading}>{loading ? '登录中…' : '登 录'}</Btn>
          <div className="login-accounts">
            <b>测试账号</b><br />
            管理员:admin / admin123<br />
            物业(101):王物业 / 123456　顶楼住户(601):钱进 / 123456<br />
            其余住户密码均为 123456(李建国、张敏、刘洋、赵芳、冯雪…)
          </div>
        </div>
      </form>
    </div>
  )
}
