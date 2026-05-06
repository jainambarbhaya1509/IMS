import { useState, useEffect } from 'react'
import { getHealth, getMetrics } from '../api/client'

// Header — top bar with system health indicator and signal throughput
export default function Header() {
  const [healthy, setHealthy] = useState(null) // null=checking, true=ok, false=down
  const [totalSignals, setTotalSignals] = useState(0)

  // Check health every 10 seconds
  useEffect(() => {
    const check = () => {
      getHealth()
        .then(() => setHealthy(true))
        .catch(() => setHealthy(false))
    }
    check()
    const interval = setInterval(check, 10000)
    return () => clearInterval(interval)
  }, [])

  // Load metrics to show total signal count
  useEffect(() => {
    getMetrics()
      .then(buckets => {
        const total = buckets.reduce((sum, b) => sum + b.count, 0)
        setTotalSignals(total)
      })
      .catch(() => {})
  }, [])

  return (
    <header style={{
      height: 52,
      borderBottom: '1px solid var(--border)',
      display: 'flex',
      alignItems: 'center',
      padding: '0 24px',
      gap: 20,
      flexShrink: 0,
      background: 'var(--bg-2)',
      position: 'relative',
      zIndex: 10,
    }}>
      {/* Logo / title */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <div style={{
          width: 28,
          height: 28,
          background: 'var(--p0)',
          borderRadius: 4,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: '14px',
        }}>
          ⚡
        </div>
        <span style={{
          fontFamily: 'var(--font-display)',
          fontWeight: 800,
          fontSize: '15px',
          letterSpacing: '0.05em',
        }}>
          IMS
        </span>
        <span style={{
          fontFamily: 'var(--font-display)',
          fontSize: '13px',
          color: 'var(--text-3)',
          fontWeight: 400,
        }}>
          Incident Management System
        </span>
      </div>

      {/* Spacer */}
      <div style={{ flex: 1 }} />

      {/* Signal count */}
      {totalSignals > 0 && (
        <div style={{ fontSize: '11px', color: 'var(--text-3)' }}>
          <span style={{ color: 'var(--text-2)' }}>{totalSignals.toLocaleString()}</span> signals (24h)
        </div>
      )}

      {/* Health indicator */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        fontSize: '11px',
        color: healthy === null ? 'var(--text-3)' : healthy ? 'var(--green)' : 'var(--p0)',
      }}>
        <span style={{
          width: 6,
          height: 6,
          borderRadius: '50%',
          background: healthy === null ? 'var(--text-3)' : healthy ? 'var(--green)' : 'var(--p0)',
          display: 'inline-block',
          animation: healthy ? 'pulse 2s ease infinite' : 'none',
        }} />
        {healthy === null ? 'Checking...' : healthy ? 'System healthy' : 'Backend unreachable'}
      </div>

      {/* Live indicator */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 5,
        fontSize: '11px',
        color: 'var(--text-3)',
        background: 'var(--bg-3)',
        border: '1px solid var(--border)',
        padding: '3px 10px',
        borderRadius: 3,
      }}>
        <span style={{
          width: 5,
          height: 5,
          borderRadius: '50%',
          background: 'var(--p0)',
          display: 'inline-block',
          animation: 'pulse 1s ease infinite',
        }} />
        LIVE
      </div>
    </header>
  )
}