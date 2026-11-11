// SPDX-License-Identifier: MIT
import React from 'react'
import {
  Activity,
  ArrowDown,
  ArrowUp,
  Ban,
  Check,
  ChevronRight,
  Copy,
  Download,
  ExternalLink,
  FolderOpen,
  GitBranch,
  Globe,
  ListFilter,
  Maximize2,
  MoreHorizontal,
  PanelLeft,
  Pencil,
  PictureInPicture2,
  Pin,
  Plus,
  Power,
  RefreshCw,
  ScrollText,
  Search,
  Server,
  ShieldCheck,
  ShoppingCart,
  SlidersHorizontal,
  Trash2,
  TriangleAlert,
  Upload,
  X,
  Zap,
  type LucideIcon,
} from 'lucide-react'
import anyproxyMark from '../assets/anyproxy-mark.svg'

export type IconName =
  | 'power'
  | 'plug'
  | 'list'
  | 'proxy'
  | 'rules'
  | 'settings'
  | 'plus'
  | 'more'
  | 'chevron'
  | 'search'
  | 'cart'
  | 'trash'
  | 'filter'
  | 'refresh'
  | 'check'
  | 'warn'
  | 'arrow-up'
  | 'arrow-down'
  | 'copy'
  | 'pencil'
  | 'upload'
  | 'download'
  | 'close'
  | 'minimize'
  | 'maximize'
  | 'pin'
  | 'sidebar'
  | 'folder'
  | 'zap'
  | 'shield'
  | 'globe'
  | 'ban'
  | 'external'

const ICON_MAP: Record<IconName, LucideIcon> = {
  power: Power,
  plug: Activity,
  list: ScrollText,
  proxy: Server,
  rules: GitBranch,
  settings: SlidersHorizontal,
  plus: Plus,
  more: MoreHorizontal,
  chevron: ChevronRight,
  search: Search,
  cart: ShoppingCart,
  trash: Trash2,
  filter: ListFilter,
  refresh: RefreshCw,
  check: Check,
  warn: TriangleAlert,
  'arrow-up': ArrowUp,
  'arrow-down': ArrowDown,
  copy: Copy,
  pencil: Pencil,
  upload: Upload,
  download: Download,
  close: X,
  minimize: PictureInPicture2,
  maximize: Maximize2,
  pin: Pin,
  sidebar: PanelLeft,
  folder: FolderOpen,
  zap: Zap,
  shield: ShieldCheck,
  globe: Globe,
  ban: Ban,
  external: ExternalLink,
}

export function Icon({ name, className = 'icon' }: { name: IconName; className?: string }) {
  const Component = ICON_MAP[name]
  return <Component className={className} strokeWidth={1.85} aria-hidden="true" />
}

export function LogoMark() {
  return <img className="mark" src={anyproxyMark} width="22" height="22" alt="" aria-hidden="true" />
}
