import { useId } from 'react'

/** The Modbus Studio mark (same drawing as build/appicon.png, simplified for small sizes). */
export function Logo({ size = 24 }: { size?: number }) {
  const id = useId()
  return (
    <svg className="logo" width={size} height={size} viewBox="0 0 1024 1024" aria-hidden>
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#2b7de9" />
          <stop offset="1" stopColor="#0848a0" />
        </linearGradient>
      </defs>
      <rect x="64" y="64" width="896" height="896" rx="170" fill={`url(#${id})`} />
      <rect x="170" y="330" width="684" height="84" rx="42" fill="#fff" />
      {[290, 512, 734].map((x, i) => (
        <g key={x}>
          <rect x={x - 32} y="410" width="64" height="118" fill="#fff" />
          <rect x={x - 98} y="524" width="196" height="196" rx="40" fill={i === 1 ? '#ffc542' : '#fff'} />
        </g>
      ))}
    </svg>
  )
}
