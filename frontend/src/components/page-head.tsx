// SPDX-License-Identifier: MIT
import React from 'react'

export function PageHead({
  title,
  sub,
  children,
}: {
  title: string
  sub?: string
  children?: React.ReactNode
}) {
  return (
    <header className="pagehead drag">
      <div>
        <h1>{title}</h1>
        {sub && <div className="sub">{sub}</div>}
      </div>
      <span className="spacer" />
      <div className="row nodrag">{children}</div>
    </header>
  )
}
