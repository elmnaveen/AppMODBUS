import { useEffect, useState } from 'react'

export type Theme = 'light' | 'dark'
const KEY = 'modbus-studio.theme'

function initial(): Theme {
  try {
    const saved = localStorage.getItem(KEY)
    if (saved === 'light' || saved === 'dark') return saved
  } catch {
    /* storage unavailable */
  }
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

/** Light/dark theme stored per machine. */
export function useTheme(): [Theme, () => void, (t: Theme) => void] {
  const [theme, setTheme] = useState<Theme>(initial)
  useEffect(() => {
    document.documentElement.dataset.theme = theme
    try {
      localStorage.setItem(KEY, theme)
    } catch {
      /* ignore */
    }
  }, [theme])
  return [theme, () => setTheme((t) => (t === 'dark' ? 'light' : 'dark')), setTheme]
}

export type MenuStyle = 'toolbar' | 'menubar' | 'modbuspoll'
const MENU_KEY = 'modbus-studio.menuStyle'

/** Grouped toolbar, classic menu bar, or Modbus Poll-style menus and toolbar; stored per machine. */
export function useMenuStyle(): [MenuStyle, (m: MenuStyle) => void] {
  const [style, setStyle] = useState<MenuStyle>(() => {
    try {
      const v = localStorage.getItem(MENU_KEY)
      return v === 'menubar' || v === 'modbuspoll' ? v : 'toolbar'
    } catch {
      return 'toolbar'
    }
  })
  useEffect(() => {
    try {
      localStorage.setItem(MENU_KEY, style)
    } catch {
      /* ignore */
    }
  }, [style])
  return [style, setStyle]
}
