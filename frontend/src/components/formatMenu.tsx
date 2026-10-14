import type { FormatInfo } from '../types'
import type { MenuEntry } from './Menu'

export interface FormatGroup {
  group: string
  /** The current format is in this group. */
  here: boolean
  items: MenuEntry[]
}

/**
 * Data formats as menu entries, per width (16/32/64-bit). Formats that differ
 * only in byte order become a submenu, e.g. Float ▸ AB CD / CD AB / BA DC / DC BA.
 */
export function formatGroups(formats: FormatInfo[], current: string, pick: (id: string) => void): FormatGroup[] {
  return ['16-bit', '32-bit', '64-bit'].map((group) => {
    // "Float CD AB" -> type "Float", byte order "CD AB"; 16-bit formats have no byte order.
    const types = new Map<string, { id: string; order: string; label: string }[]>()
    for (const f of formats.filter((x) => x.group === group)) {
      const m = f.label.match(/^(.*?)\s+([A-H]{2}(?: [A-H]{2})+)$/)
      const key = m ? m[1] : f.label
      types.set(key, [...(types.get(key) ?? []), { id: f.id, order: m ? m[2] : '', label: f.label }])
    }
    const items = [...types].map(([type, fs]): MenuEntry => {
      if (fs.length === 1 && !fs[0].order) return { label: fs[0].label, radio: true, checked: current === fs[0].id, onSelect: () => pick(fs[0].id) }
      return {
        type: 'sub', label: type, icon: fs.some((f) => f.id === current) ? <span className="wm-dot" /> : undefined,
        items: fs.map((f): MenuEntry => ({ label: f.order, radio: true, checked: current === f.id, onSelect: () => pick(f.id) })),
      }
    })
    return { group, here: formats.some((f) => f.group === group && f.id === current), items }
  })
}
