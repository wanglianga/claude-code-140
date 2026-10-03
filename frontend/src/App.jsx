import React, { useEffect, useState } from 'react'
import { api, clearToken, getToken, setToken, ROLE_LABEL } from './lib/api'
import { ToastProvider, useToast } from './components/ui'
import Login from './pages/Login'
import BuildingList from './pages/BuildingList'
import BuildingDetail from './pages/BuildingDetail'

function Shell() {
  const { notify } = useToast()
  const [user, setUser] = useState(null)
  const [route, setRoute] = useState({ name: 'list' })
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!getToken()) {
      setLoading(false)
      return
    }
    api('GET', '/me')
      .then((u) => setUser(u))
      .catch(() => clearToken())
      .finally(() => setLoading(false))
  }, [])

  const onLogin = async (phone, password) => {
    const r = await api('POST', '/auth/login', { phone, password })
    setToken(r.token)
    setUser(r.user)
    setRoute({ name: 'list' })
  }
  const logout = () => {
    clearToken()
    setUser(null)
    notify('已退出登录')
  }

  if (loading) return <div className="container"><div className="muted">加载中…</div></div>
  if (!user) return <Login onLogin={onLogin} />

  return (
    <>
      <div className="topbar">
        <span className="brand">☀️ 屋顶光伏认购结算与维修平台</span>
        <span className="spacer" />
        <span className="small">
          {user.name} <span className="badge green">{ROLE_LABEL[user.role] || user.role}</span>
        </span>
        {route.name !== 'list' && (
          <button onClick={() => setRoute({ name: 'list' })}>楼栋列表</button>
        )}
        <button onClick={logout}>退出</button>
      </div>
      {route.name === 'list' && (
        <BuildingList user={user} onOpen={(id) => setRoute({ name: 'building', id })} />
      )}
      {route.name === 'building' && (
        <BuildingDetail user={user} id={route.id} />
      )}
    </>
  )
}

export default function App() {
  return (
    <ToastProvider>
      <Shell />
    </ToastProvider>
  )
}
