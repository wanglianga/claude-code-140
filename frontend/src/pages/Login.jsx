import React, { useState } from 'react'
import { handleErr, useToast } from '../components/ui'

export default function Login({ onLogin }) {
  const { notify } = useToast()
  const [phone, setPhone] = useState('13800000001')
  const [password, setPassword] = useState('property123')
  const [busy, setBusy] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    setBusy(true)
    try {
      await handleErr(onLogin(phone, password), notify)
    } finally {
      setBusy(false)
    }
  }

  const quick = (p, pw) => {
    setPhone(p)
    setPassword(pw)
  }

  return (
    <div className="login-wrap">
      <div className="card">
        <h1>☀️ 屋顶光伏平台</h1>
        <p className="muted">居民楼屋顶光伏认购、发电结算与维修协同</p>
        <form onSubmit={submit}>
          <label className="field">
            <span>手机号</span>
            <input value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="手机号" />
          </label>
          <label className="field">
            <span>密码</span>
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
          </label>
          <button className="primary" style={{ width: '100%' }} disabled={busy}>
            {busy ? '登录中…' : '登 录'}
          </button>
        </form>
        <div className="login-accounts">
          <div><b>演示账号（点击可快速填入）：</b></div>
          <div>👑 管理员 <code>13800000000 / admin123</code></div>
          <button className="sm" onClick={() => quick('13800000000', 'admin123')}>用管理员登录</button>
          <div>🏢 物业王经理 <code>13800000001 / property123</code></div>
          <button className="sm" onClick={() => quick('13800000001', 'property123')}>用物业登录</button>
          <div>🏠 顶楼住户张伟 <code>13900000001 / resident123</code></div>
          <button className="sm" onClick={() => quick('13900000001', 'resident123')}>用住户登录</button>
        </div>
      </div>
    </div>
  )
}
