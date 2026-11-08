// SPDX-License-Identifier: MIT
import React from 'react'
import { Icon, type IconName } from './icons'

export function EmptyState({
  icon,
  title,
  text,
  children,
}: {
  icon: IconName
  title: string
  text?: string
  children?: React.ReactNode
}) {
  return (
    <div className="emptystate">
      <span className="glyph">
        <Icon name={icon} className="icon lg" />
      </span>
      <b>{title}</b>
      {text && <p>{text}</p>}
      {children && <div className="row">{children}</div>}
    </div>
  )
}
