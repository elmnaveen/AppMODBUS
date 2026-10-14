// Minimal inline icon set (24x24, stroke based).
import type { ReactNode } from 'react'

const I = ({ children }: { children: ReactNode }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.9} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
    {children}
  </svg>
)

export const IconPlug = () => <I><path d="M9 7V3M15 7V3M7 7h10v4a5 5 0 0 1-10 0V7zM12 16v5" /></I>
export const IconPlus = () => <I><path d="M12 5v14M5 12h14" /></I>
export const IconPlay = () => <I><path d="M7 5l12 7-12 7V5z" /></I>
export const IconStop = () => <I><rect x="6" y="6" width="12" height="12" rx="1.5" /></I>
export const IconOnce = () => <I><path d="M4 12a8 8 0 1 0 2.3-5.6M4 4v4h4" /></I>
export const IconGear = () => <I><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" /></I>
export const IconPen = () => <I><path d="M4 20h4L19 9l-4-4L4 16v4zM14 6l4 4" /></I>
export const IconList = () => <I><path d="M8 6h12M8 12h12M8 18h12M4 6h.01M4 12h.01M4 18h.01" /></I>
export const IconServer = () => <I><rect x="3" y="4" width="18" height="7" rx="2" /><rect x="3" y="13" width="18" height="7" rx="2" /><path d="M7 7.5h.01M7 16.5h.01" /></I>
export const IconSun = () => <I><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></I>
export const IconMoon = () => <I><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" /></I>
export const IconTabs = () => <I><rect x="3" y="5" width="18" height="15" rx="2" /><path d="M3 9h18M9 5v4" /></I>
export const IconTile = () => <I><rect x="3" y="3" width="8" height="8" rx="1.5" /><rect x="13" y="3" width="8" height="8" rx="1.5" /><rect x="3" y="13" width="8" height="8" rx="1.5" /><rect x="13" y="13" width="8" height="8" rx="1.5" /></I>
export const IconX = () => <I><path d="M6 6l12 12M18 6L6 18" /></I>
export const IconTrash = () => <I><path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3" /></I>
export const IconPause = () => <I><path d="M8 5v14M16 5v14" /></I>
export const IconSave = () => <I><path d="M5 3h11l3 3v15H5V3zM8 3v5h7V3M8 21v-7h8v7" /></I>
export const IconBolt = () => <I><path d="M13 2L4 14h7l-1 8 9-12h-7l1-8z" /></I>
export const IconRefresh = () => <I><path d="M20 11a8 8 0 0 0-14.3-4.9L4 8M4 4v4h4M4 13a8 8 0 0 0 14.3 4.9L20 16M20 20v-4h-4" /></I>
export const IconInfo = () => <I><circle cx="12" cy="12" r="9" /><path d="M12 11v5M12 8h.01" /></I>
export const IconDownload = () => <I><path d="M12 4v11M7 10l5 5 5-5M5 20h14" /></I>
export const IconExternal = () => <I><path d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5" /></I>
export const IconFile = () => <I><path d="M14 3H6a1 1 0 0 0-1 1v16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V8l-5-5zM14 3v5h5" /></I>
export const IconFolder = () => <I><path d="M3 6a1 1 0 0 1 1-1h5l2 2h9a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V6z" /></I>
export const IconChevron = () => <I><path d="M6 9l6 6 6-6" /></I>
export const IconTag = () => <I><path d="M3 12V4a1 1 0 0 1 1-1h8l9 9-9 9-9-9zM7.5 7.5h.01" /></I>
export const IconBook = () => <I><path d="M4 4.5A1.5 1.5 0 0 1 5.5 3H20v15H5.5A1.5 1.5 0 0 0 4 19.5v-15zM4 19.5A1.5 1.5 0 0 0 5.5 21H20v-3" /></I>
export const IconUpload = () => <I><path d="M12 20V9M7 14l5-5 5 5M5 4h14" /></I>
export const IconCopy = () => <I><rect x="8" y="8" width="12" height="12" rx="2" /><path d="M16 8V5a1 1 0 0 0-1-1H5a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h3" /></I>
export const IconDatabase = () => <I><ellipse cx="12" cy="5" rx="8" ry="3" /><path d="M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3" /></I>
export const IconChart = () => <I><path d="M4 4v16h16" /><path d="M7 15l4-5 3 3 5-6" /></I>
export const IconPulse = () => <I><path d="M3 12h4l3-8 4 16 3-8h4" /></I>
export const IconBell = () => <I><path d="M6 8a6 6 0 1 1 12 0c0 7 3 9 3 9H3s3-2 3-9M10.3 21a1.9 1.9 0 0 0 3.4 0" /></I>
export const IconCheck = () => <I><path d="M5 12l5 5L20 7" /></I>
