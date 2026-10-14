import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type MouseEvent as RMouseEvent, type ReactNode } from 'react'
import { IconChevron } from './Icons'

/** One row of a menu. Menus are plain data so they can nest to any depth. */
export type MenuEntry =
  | {
      type?: 'item'
      label: string
      icon?: ReactNode
      /** Shortcut text shown on the right. */
      hint?: string
      /** Tick (checkbox item) or, with radio, a dot (one-of-many item). */
      checked?: boolean
      radio?: boolean
      disabled?: boolean
      title?: string
      onSelect: () => void
    }
  | { type: 'sub'; label: string; icon?: ReactNode; disabled?: boolean; items: MenuEntry[] }
  | { type: 'sep' }
  | { type: 'header'; label: string }

/** Box the menu opens against: below it (menu bar, toolbar) or beside it (submenu, right-click). */
export interface Anchor {
  left: number
  top: number
  right: number
  bottom: number
  place: 'below' | 'right'
}

export const pointAnchor = (x: number, y: number): Anchor => ({ left: x, right: x, top: y, bottom: y, place: 'right' })

const selectable = (e: MenuEntry) => (e.type === undefined || e.type === 'item' || e.type === 'sub') && !e.disabled

interface PanelProps {
  entries: MenuEntry[]
  anchor: Anchor
  /** Closes the whole menu tree (after an item is chosen, or Escape on the top level). */
  onClose: () => void
  /** Closes only this level (Left / Escape in a submenu). Absent on the top level. */
  onBack?: () => void
  /** Left/Right on the top level, so a menu bar can move to the next menu. */
  onSide?: (dir: -1 | 1) => void
  /** Highlight the first item on open (keyboard navigation). */
  focusFirst?: boolean
  /** The pointer entered this panel (a submenu tells its parent to keep it open). */
  onEnter?: () => void
}

/**
 * A Windows-style popup menu. Hover or Right opens a submenu, Up/Down move,
 * Enter chooses, Left/Escape go back a level. Submenus flip to the left and
 * menus shift up when they would leave the window.
 */
export function MenuPanel({ entries, anchor, onClose, onBack, onSide, focusFirst, onEnter }: PanelProps) {
  const ref = useRef<HTMLDivElement>(null)
  const itemRefs = useRef<(HTMLDivElement | null)[]>([])
  // Measured at the top-left first, so the size is not squeezed by the window edge.
  const [pos, setPos] = useState<CSSProperties>({ left: 0, top: 0, visibility: 'hidden' })
  const [active, setActive] = useState(() => (focusFirst ? entries.findIndex(selectable) : -1))
  const [sub, setSub] = useState<{ index: number; kbd: boolean } | null>(null)
  const timer = useRef<number>()

  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const w = el.offsetWidth
    const h = el.offsetHeight
    const W = window.innerWidth
    const H = window.innerHeight
    let left: number
    let top: number
    if (anchor.place === 'below') {
      left = Math.min(anchor.left, W - w - 4)
      top = anchor.bottom
      if (top + h > H - 4) top = Math.max(4, H - h - 4)
    } else {
      // Open to the right; flip left when that overflows; failing both, hug the right edge.
      left = anchor.right + w <= W - 4 ? anchor.right : anchor.left - w >= 4 ? anchor.left - w : W - w - 4
      top = Math.min(anchor.top, H - h - 4)
    }
    setPos({ left: Math.max(4, left), top: Math.max(4, top) })
  }, [anchor.left, anchor.top, anchor.right, anchor.bottom, anchor.place])

  useEffect(() => () => window.clearTimeout(timer.current), [])

  const choose = (i: number, kbd: boolean) => {
    const e = entries[i]
    if (!e || !selectable(e)) return
    if (e.type === 'sub') {
      window.clearTimeout(timer.current)
      setSub({ index: i, kbd })
    } else if (e.type === undefined || e.type === 'item') {
      onClose()
      e.onSelect()
    }
  }

  // Only the deepest open level listens to the keyboard.
  useEffect(() => {
    if (sub) return
    const step = (dir: 1 | -1) => setActive((from) => {
      const n = entries.length
      for (let k = 1; k <= n; k++) {
        const i = (((from < 0 ? (dir === 1 ? -1 : 0) : from) + dir * k) % n + n) % n
        if (selectable(entries[i])) return i
      }
      return from
    })
    const onKey = (e: KeyboardEvent) => {
      const cur = entries[active]
      switch (e.key) {
        case 'ArrowDown': step(1); break
        case 'ArrowUp': step(-1); break
        case 'ArrowRight':
          if (cur?.type === 'sub' && !cur.disabled) choose(active, true)
          else if (onSide) onSide(1)
          else return
          break
        case 'ArrowLeft':
          if (onBack) onBack()
          else if (onSide) onSide(-1)
          else return
          break
        case 'Enter':
        case ' ':
          if (active >= 0) choose(active, true)
          break
        case 'Escape':
          if (onBack) onBack()
          else onClose()
          break
        case 'Home': setActive(entries.findIndex(selectable)); break
        case 'End': for (let i = entries.length - 1; i >= 0; i--) if (selectable(entries[i])) { setActive(i); break } break
        default: return
      }
      e.preventDefault()
      e.stopPropagation()
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  })

  const hover = (i: number) => {
    setActive(i)
    window.clearTimeout(timer.current)
    const e = entries[i]
    if (sub?.index === i) return
    // A short delay lets the pointer cross other rows on its way into an open submenu.
    timer.current = window.setTimeout(() => setSub(e.type === 'sub' && !e.disabled ? { index: i, kbd: false } : null), sub ? 250 : 120)
  }

  let subAnchor: Anchor | null = null
  if (sub) {
    const r = itemRefs.current[sub.index]?.getBoundingClientRect()
    if (r) subAnchor = { left: r.left - 2, right: r.right + 2, top: r.top - 5, bottom: r.bottom, place: 'right' }
  }
  const subEntry = sub ? entries[sub.index] : null

  return (
    <div className="wm" ref={ref} style={pos} role="menu" onContextMenu={(e) => e.preventDefault()} onMouseEnter={() => { window.clearTimeout(timer.current); onEnter?.() }}>
      {entries.map((e, i) => {
        if (e.type === 'sep') return <div key={i} className="wm-sep" role="separator" />
        if (e.type === 'header') return <div key={i} className="wm-head">{e.label}</div>
        const isSub = e.type === 'sub'
        const item = !isSub ? e : null
        return (
          <div
            key={i}
            ref={(el) => { itemRefs.current[i] = el }}
            className={`wm-item${active === i ? ' active' : ''}${e.disabled ? ' disabled' : ''}${sub?.index === i ? ' open' : ''}`}
            role={item && item.checked !== undefined ? (item.radio ? 'menuitemradio' : 'menuitemcheckbox') : 'menuitem'}
            aria-checked={item?.checked}
            aria-disabled={e.disabled}
            aria-haspopup={isSub ? 'menu' : undefined}
            title={item?.title}
            onMouseEnter={() => hover(i)}
            onMouseLeave={() => !sub && setActive(-1)}
            onClick={() => choose(i, false)}
          >
            <span className="wm-gut">
              {item?.checked ? (item.radio ? <span className="wm-dot" /> : <span className="wm-tick">✓</span>) : e.icon}
            </span>
            <span className="wm-label">{e.label}</span>
            <span className="wm-hint">{item?.hint}</span>
            <span className="wm-arrow">{isSub && <IconChevron />}</span>
          </div>
        )
      })}
      {subEntry?.type === 'sub' && subAnchor && (
        <MenuPanel
          key={sub!.index}
          entries={subEntry.items}
          anchor={subAnchor}
          onClose={onClose}
          onBack={() => setSub(null)}
          onEnter={() => { window.clearTimeout(timer.current); setActive(sub!.index) }}
          focusFirst={sub!.kbd}
        />
      )}
    </div>
  )
}

interface DropdownProps {
  label: string
  icon?: ReactNode
  entries: MenuEntry[]
  /** Extra button classes, e.g. "sm". */
  className?: string
  title?: string
  disabled?: boolean
}

/** A toolbar button that opens a menu below it. */
export function Dropdown({ label, icon, entries, className, title, disabled }: DropdownProps) {
  const [anchor, setAnchor] = useState<Anchor | null>(null)
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!anchor) return
    const onDown = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && setAnchor(null)
    window.addEventListener('mousedown', onDown)
    return () => window.removeEventListener('mousedown', onDown)
  }, [anchor])
  const toggle = (e: RMouseEvent<HTMLButtonElement>) => {
    if (anchor) return setAnchor(null)
    const r = e.currentTarget.getBoundingClientRect()
    setAnchor({ left: r.left, right: r.right, top: r.top, bottom: r.bottom + 2, place: 'below' })
  }
  return (
    <div className="dropdown" ref={ref}>
      <button className={`btn${className ? ' ' + className : ''}${anchor ? ' on' : ''}`} title={title} disabled={disabled} aria-haspopup="menu" aria-expanded={!!anchor} onClick={toggle}>
        {icon}{label}<IconChevron />
      </button>
      {anchor && <MenuPanel entries={entries} anchor={anchor} onClose={() => setAnchor(null)} />}
    </div>
  )
}

/** A right-click menu at a point; closes on outside click. */
export function ContextMenu({ x, y, entries, onClose }: { x: number; y: number; entries: MenuEntry[]; onClose: () => void }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const onDown = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && onClose()
    window.addEventListener('mousedown', onDown)
    return () => window.removeEventListener('mousedown', onDown)
  }, [onClose])
  return (
    <div ref={ref}>
      <MenuPanel entries={entries} anchor={pointAnchor(x, y)} onClose={onClose} />
    </div>
  )
}
