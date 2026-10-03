import { useState } from 'react'
import { getUser, clearAuth } from './api'
import Login from './pages/Login.jsx'
import Dashboard from './pages/Dashboard.jsx'
import BuildingDetail from './pages/BuildingDetail.jsx'

export default function App() {
  const [user, setUser] = useState(getUser())
  const [buildingId, setBuildingId] = useState(null)

  if (!user) {
    return <Login onLogin={(u) => setUser(u)} />
  }

  const logout = () => {
    clearAuth()
    setUser(null)
    setBuildingId(null)
  }

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">☀️ 屋顶光伏平台</div>
        <nav>
          <button className={buildingId ? '' : 'on'} onClick={() => setBuildingId(null)}>
            🏠 {user.kind === 'ADMIN' ? '楼栋总览' : '我的楼栋'}
          </button>
          {buildingId && (
            <>
              <div style={{ padding: '6px 12px 2px', fontSize: 11, color: '#7ea093' }}>楼栋工作台</div>
              <button className="on" onClick={() => setBuildingId(null)}>⬅ 返回列表</button>
            </>
          )}
        </nav>
        <div className="me">
          <div>{user.kind === 'ADMIN' ? '🛠 平台管理员' : user.role === 'PROPERTY' ? '🏢 物业' : '👤 住户'}</div>
          <b>{user.name}</b>
          {user.unit_no && <span> · {user.unit_no}</span>}
          <div style={{ marginTop: 8 }}>
            <button className="btn btn-sm" onClick={logout}>退出登录</button>
          </div>
        </div>
      </aside>
      <main className="main">
        {buildingId ? (
          <BuildingDetail user={user} buildingId={buildingId} onBack={() => setBuildingId(null)} />
        ) : (
          <Dashboard user={user} onOpen={setBuildingId} />
        )}
      </main>
    </div>
  )
}
