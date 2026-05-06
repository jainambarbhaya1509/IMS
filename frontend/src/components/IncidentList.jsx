import { formatDistanceToNow } from 'date-fns'
import SeverityBadge from './SeverityBadge'
import StatusBadge from './StatusBadge'

// IncidentList — the live feed panel on the left
// Shows all work items sorted by severity (P0 first)
// Clicking a row opens the detail panel on the right
export default function IncidentList({ items, selectedId, onSelect, loading }) {
  // Sort: P0 first, then P1, then P2
  // Within same severity, sort by newest first
  const severityOrder = { P0: 0, P1: 1, P2: 2 }
  const sorted = [...items].sort((a, b) => {
    const sev = severityOrder[a.severity] - severityOrder[b.severity]
    if (sev !== 0) return sev
    return new Date(b.start_time) - new Date(a.start_time)
  })

  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      height: '100%',
      overflow: 'hidden',
    }}>
      {/* Header */}
      <div style={{
        padding: '16px 20px',
        borderBottom: '1px solid var(--border)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        flexShrink: 0,
      }}>
        <span style={{
          fontFamily: 'var(--font-display)',
          fontSize: '14px',
          fontWeight: 700,
          letterSpacing: '0.05em',
          color: 'var(--text)',
        }}>
          ACTIVE INCIDENTS
        </span>
        <span style={{
          fontSize: '11px',
          color: 'var(--text-3)',
          background: 'var(--bg-3)',
          padding: '2px 8px',
          borderRadius: '3px',
          border: '1px solid var(--border)',
        }}>
          {items.length} total
        </span>
      </div>

      {/* List */}
      <div style={{ flex: 1, overflowY: 'auto' }}>
        {loading && items.length === 0 && (
          <div style={{ padding: '40px 20px', color: 'var(--text-3)', textAlign: 'center' }}>
            Loading incidents...
          </div>
        )}

        {!loading && items.length === 0 && (
          <div style={{ padding: '40px 20px', color: 'var(--text-3)', textAlign: 'center' }}>
            <div style={{ fontSize: '24px', marginBottom: 8 }}>✓</div>
            <div>No active incidents</div>
          </div>
        )}

        {sorted.map((item, i) => (
          <IncidentRow
            key={item.id}
            item={item}
            selected={item.id === selectedId}
            onClick={() => onSelect(item)}
            index={i}
          />
        ))}
      </div>
    </div>
  )
}

// A single row in the incident list
function IncidentRow({ item, selected, onClick, index }) {
  const timeAgo = formatDistanceToNow(new Date(item.start_time), { addSuffix: true })

  // Left border color by severity — instant visual triage
  const borderColor = {
    P0: 'var(--p0)',
    P1: 'var(--p1)',
    P2: 'var(--p2)',
  }[item.severity] || 'var(--border-2)'

  return (
    <div
      onClick={onClick}
      className="animate-in"
      style={{
        padding: '14px 20px',
        borderBottom: '1px solid var(--border)',
        borderLeft: `3px solid ${selected ? borderColor : 'transparent'}`,
        background: selected ? 'var(--bg-3)' : 'transparent',
        cursor: 'pointer',
        transition: 'background 0.1s, border-color 0.1s',
        animationDelay: `${index * 0.04}s`,
        opacity: 0,
      }}
      onMouseEnter={e => {
        if (!selected) e.currentTarget.style.background = 'var(--bg-2)'
      }}
      onMouseLeave={e => {
        if (!selected) e.currentTarget.style.background = 'transparent'
      }}
    >
      {/* Top row: component name + severity */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
        <span style={{
          fontFamily: 'var(--font-display)',
          fontWeight: 700,
          fontSize: '13px',
          color: 'var(--text)',
        }}>
          {item.component_id}
        </span>
        <SeverityBadge severity={item.severity} />
      </div>

      {/* Bottom row: status + signal count + time */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <StatusBadge status={item.status} />
        <div style={{ display: 'flex', gap: 12, color: 'var(--text-3)', fontSize: '11px' }}>
          <span>{item.signal_count} signals</span>
          <span>{timeAgo}</span>
        </div>
      </div>

      {/* MTTR badge if closed */}
      {item.mttr_minutes && (
        <div style={{ marginTop: 6 }}>
          <span style={{
            fontSize: '11px',
            color: 'var(--green)',
            background: 'var(--green-bg)',
            padding: '1px 6px',
            borderRadius: '3px',
          }}>
            MTTR: {item.mttr_minutes.toFixed(1)}m
          </span>
        </div>
      )}
    </div>
  )
}