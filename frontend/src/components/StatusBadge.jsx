export default function StatusBadge({ status }) {
  const map = {
    OPEN:          '#ff3b3b',
    INVESTIGATING: '#ff8c00',
    RESOLVED:      '#4a9eff',
    CLOSED:        '#00d084',
  }
  const color = map[status] || map.OPEN
  const live = status === 'OPEN' || status === 'INVESTIGATING'
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5, color, fontSize: '11px', fontWeight: 600, letterSpacing: '0.08em' }}>
      {live && <span style={{ width: 6, height: 6, borderRadius: '50%', background: color, display: 'inline-block', animation: 'pulse 1.5s ease infinite' }} />}
      {status}
    </span>
  )
}