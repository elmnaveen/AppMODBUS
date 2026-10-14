import { useEffect, useRef, useState } from 'react'
import { MenuPanel, type Anchor, type MenuEntry } from './Menu'

export interface BarMenu {
  /** Title shown in the bar. */
  title: string
  /** Index of the Alt access letter in the title (default 0). */
  accel?: number
  entries: MenuEntry[]
}

/**
 * A classic Windows menu bar. Click a title to open it; while one is open,
 * hovering another title switches to it. Holding Alt underlines the access
 * keys and Alt+letter opens that menu; Left/Right move between menus.
 */
export function MenuBar({ menus }: { menus: BarMenu[] }) {
  const [open, setOpen] = useState<{ index: number; kbd: boolean } | null>(null)
  const [alt, setAlt] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const titles = useRef<(HTMLButtonElement | null)[]>([])

  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === 'Alt') { setAlt(true); return }
      if (e.altKey && !e.ctrlKey && !e.metaKey && e.key.length === 1) {
        const i = menus.findIndex((m) => m.title[m.accel ?? 0].toLowerCase() === e.key.toLowerCase())
        if (i >= 0) { e.preventDefault(); setOpen({ index: i, kbd: true }) }
      }
    }
    const up = (e: KeyboardEvent) => e.key === 'Alt' && setAlt(false)
    const blur = () => setAlt(false)
    window.addEventListener('keydown', down)
    window.addEventListener('keyup', up)
    window.addEventListener('blur', blur)
    return () => {
      window.removeEventListener('keydown', down)
      window.removeEventListener('keyup', up)
      window.removeEventListener('blur', blur)
    }
  }, [menus])

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && setOpen(null)
    window.addEventListener('mousedown', onDown)
    return () => window.removeEventListener('mousedown', onDown)
  }, [open])

  let anchor: Anchor | null = null
  if (open) {
    const r = titles.current[open.index]?.getBoundingClientRect()
    if (r) anchor = { left: r.left, right: r.right, top: r.top, bottom: r.bottom + 1, place: 'below' }
  }

  return (
    <div className={`menubar${alt || open?.kbd ? ' keys' : ''}`} ref={ref} role="menubar">
      {menus.map((m, i) => (
        <button
          key={m.title}
          ref={(el) => { titles.current[i] = el }}
          className={`mb-title${open?.index === i ? ' on' : ''}`}
          aria-haspopup="menu"
          aria-expanded={open?.index === i}
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => setOpen(open?.index === i ? null : { index: i, kbd: false })}
          onMouseEnter={() => open && open.index !== i && setOpen({ index: i, kbd: false })}
        >
          {m.title.slice(0, m.accel ?? 0)}<u>{m.title[m.accel ?? 0]}</u>{m.title.slice((m.accel ?? 0) + 1)}
        </button>
      ))}
      {open && anchor && (
        <MenuPanel
          key={open.index}
          entries={menus[open.index].entries}
          anchor={anchor}
          focusFirst={open.kbd}
          onClose={() => setOpen(null)}
          onSide={(dir) => setOpen({ index: (open.index + dir + menus.length) % menus.length, kbd: true })}
        />
      )}
    </div>
  )
}
