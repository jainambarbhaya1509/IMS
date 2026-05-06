export default function SeverityBadge({ severity }) {
  const styles = {
    P0: { color: '#ff3b3b', background: '#1a0808', border: '1px solid #3d1010' },
    P1: { color: '#ff8c00', background: '#1a1000', border: '1px solid #3d2800' },
    P2: { color: '#f5c400', background: '#1a1600', border: '1px solid #3d3400' },
  }
  return (
    <span style={{
      ...(styles[severity] || styles.P2),
      padding: '2px 8px', borderRadius: '3px',
      fontSize: '11px', fontWeight: 700, letterSpacing: '0.05em',
    }}>
      {severity}
    </span>
  )
}