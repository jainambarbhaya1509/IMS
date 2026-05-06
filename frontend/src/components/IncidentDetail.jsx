import { useState, useEffect } from 'react'
import { formatDistanceToNow, format } from 'date-fns'
import { getSignals, transitionStatus } from '../api/client'
import SeverityBadge from './SeverityBadge'
import StatusBadge from './StatusBadge'
import RCAForm from './RCAForm'

// Valid next states for each current state
// This mirrors the backend state machine so the UI only shows valid actions
const NEXT_STATUS = {
  OPEN:          'INVESTIGATING',
  INVESTIGATING: 'RESOLVED',
  RESOLVED:      null,  // needs RCA form, not a direct button
  CLOSED:        null,  // terminal state
}

const ACTION_LABEL = {
  INVESTIGATING: 'Start Investigating',
  RESOLVED:      'Mark as Resolved',
}

// IncidentDetail — right panel showing full incident info
// Loads raw signals from MongoDB, shows status controls and RCA form
export default function IncidentDetail({ item, onStatusChange }) {
  const [signals, setSignals] = useState([])
  const [loadingSignals, setLoadingSignals] = useState(false)
  const [transitioning, setTransitioning] = useState(false)
  const [error, setError] = useState(null)

  // Load signals whenever selected incident changes
  useEffect(() => {
    if (!item) return
    setLoadingSignals(true)
    setSignals([])
    getSignals(item.id)
      .then(setSignals)
      .catch(e => console.error('Failed to load signals:', e))
      .finally(() => setLoadingSignals(false))
  }, [item?.id])

  if (!item) {
    return (
      <div style={{
        flex: 1,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: 'var(--text-3)',
        flexDirection: 'column',
        gap: 12,
      }}>
        <div style={{ fontSize: '32px', opacity: 0.3 }}>⬡</div>
        <div>Select an incident to view details</div>
      </div>
    )
  }

  const nextStatus = NEXT_STATUS[item.status]

  const handleTransition = async () => {
    if (!nextStatus) return
    setTransitioning(true)
    setError(null)
    try {
      await transitionStatus(item.id, nextStatus)
      onStatusChange()
    } catch (e) {
      setError(e.response?.data || e.message)
    } finally {
      setTransitioning(false)
    }
  }

  return (
    <div style={{
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      height: '100%',
      overflow: 'hidden',
    }}>
      {/* ── Header ─────────────────────────────── */}
      <div style={{
        padding: '16px 24px',
        borderBottom: '1px solid var(--border)',
        flexShrink: 0,
      }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 16 }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 6 }}>
              <span style={{
                fontFamily: 'var(--font-display)',
                fontSize: '18px',
                fontWeight: 800,
                color: 'var(--text)',
              }}>
                {item.component_id}
              </span>
              <SeverityBadge severity={item.severity} />
            </div>
            <div style={{ display: 'flex', gap: 16, alignItems: 'center' }}>
              <StatusBadge status={item.status} />
              <span style={{ color: 'var(--text-3)', fontSize: '11px' }}>
                Started {formatDistanceToNow(new Date(item.start_time), { addSuffix: true })}
              </span>
              <span style={{ color: 'var(--text-3)', fontSize: '11px' }}>
                {item.signal_count} signals
              </span>
              {item.mttr_minutes && (
                <span style={{
                  color: 'var(--green)',
                  fontSize: '11px',
                  background: 'var(--green-bg)',
                  padding: '1px 6px',
                  borderRadius: 3,
                }}>
                  MTTR: {item.mttr_minutes.toFixed(1)}m
                </span>
              )}
            </div>
          </div>

          {/* Transition button — only shown when a next state exists */}
          {nextStatus && (
            <button
              onClick={handleTransition}
              disabled={transitioning}
              style={{
                background: 'var(--bg-3)',
                border: '1px solid var(--border-2)',
                color: 'var(--text)',
                padding: '8px 16px',
                borderRadius: 'var(--radius)',
                fontSize: '12px',
                fontWeight: 600,
                letterSpacing: '0.06em',
                whiteSpace: 'nowrap',
                transition: 'border-color 0.15s',
                opacity: transitioning ? 0.6 : 1,
              }}
              onMouseEnter={e => e.currentTarget.style.borderColor = 'var(--blue)'}
              onMouseLeave={e => e.currentTarget.style.borderColor = 'var(--border-2)'}
            >
              {transitioning ? 'Updating...' : `→ ${ACTION_LABEL[nextStatus]}`}
            </button>
          )}
        </div>

        {/* Incident ID */}
        <div style={{ marginTop: 10, fontSize: '11px', color: 'var(--text-3)' }}>
          ID: <span style={{ color: 'var(--text-2)', fontFamily: 'var(--font-mono)' }}>{item.id}</span>
        </div>

        {error && (
          <div style={{
            marginTop: 10,
            padding: '8px 12px',
            background: 'var(--p0-bg)',
            border: '1px solid var(--p0-border)',
            borderRadius: 'var(--radius)',
            color: 'var(--p0)',
            fontSize: '12px',
          }}>
            ✗ {error}
          </div>
        )}
      </div>

      {/* ── Body ───────────────────────────────── */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '20px 24px', display: 'flex', flexDirection: 'column', gap: 24 }}>

        {/* RCA Form — only shown when RESOLVED and waiting for RCA */}
        {item.status === 'RESOLVED' && (
          <RCAForm
            workItemId={item.id}
            onSuccess={onStatusChange}
          />
        )}

        {/* Raw Signals from MongoDB */}
        <div>
          <div style={{
            fontSize: '11px',
            color: 'var(--text-3)',
            fontWeight: 600,
            letterSpacing: '0.1em',
            textTransform: 'uppercase',
            marginBottom: 12,
            display: 'flex',
            alignItems: 'center',
            gap: 8,
          }}>
            Raw Signals
            <span style={{
              background: 'var(--bg-3)',
              border: '1px solid var(--border)',
              padding: '1px 6px',
              borderRadius: 3,
              color: 'var(--text-2)',
            }}>
              {signals.length}
            </span>
            <span style={{ color: 'var(--text-3)', fontWeight: 400 }}>from MongoDB audit log</span>
          </div>

          {loadingSignals && (
            <div style={{ color: 'var(--text-3)', fontSize: '12px', padding: '20px 0' }}>
              Loading signals...
            </div>
          )}

          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {signals.map((sig, i) => (
              <SignalRow key={sig.id || i} signal={sig} />
            ))}
          </div>

          {!loadingSignals && signals.length === 0 && (
            <div style={{ color: 'var(--text-3)', fontSize: '12px', padding: '20px 0' }}>
              No signals found
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// A single raw signal row
function SignalRow({ signal }) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div style={{
      background: 'var(--bg-2)',
      border: '1px solid var(--border)',
      borderRadius: 'var(--radius)',
      overflow: 'hidden',
      fontSize: '12px',
    }}>
      {/* Signal summary row */}
      <div
        onClick={() => setExpanded(e => !e)}
        style={{
          padding: '8px 14px',
          display: 'flex',
          alignItems: 'center',
          gap: 12,
          cursor: 'pointer',
          userSelect: 'none',
        }}
        onMouseEnter={e => e.currentTarget.style.background = 'var(--bg-3)'}
        onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
      >
        {/* Expand toggle */}
        <span style={{
          color: 'var(--text-3)',
          fontSize: '10px',
          transform: expanded ? 'rotate(90deg)' : 'none',
          transition: 'transform 0.15s',
          display: 'inline-block',
        }}>▶</span>

        <span style={{ color: 'var(--text-3)', fontFamily: 'var(--font-mono)', fontSize: '11px' }}>
          {signal.received_at ? format(new Date(signal.received_at), 'HH:mm:ss.SSS') : '—'}
        </span>

        <span style={{ color: 'var(--text)', flex: 1 }}>
          {signal.message || 'No message'}
        </span>

        <span style={{
          fontSize: '10px',
          color: 'var(--text-3)',
          fontFamily: 'var(--font-mono)',
          maxWidth: 120,
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
        }}>
          {signal.id}
        </span>
      </div>

      {/* Expanded payload view */}
      {expanded && signal.payload && (
        <div style={{
          borderTop: '1px solid var(--border)',
          padding: '10px 14px',
          background: 'var(--bg)',
        }}>
          <div style={{
            fontSize: '11px',
            color: 'var(--text-3)',
            marginBottom: 6,
            fontWeight: 600,
            letterSpacing: '0.08em',
          }}>
            PAYLOAD
          </div>
          <pre style={{
            color: 'var(--text-2)',
            fontFamily: 'var(--font-mono)',
            fontSize: '11px',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
          }}>
            {JSON.stringify(signal.payload, null, 2)}
          </pre>
        </div>
      )}
    </div>
  )
}