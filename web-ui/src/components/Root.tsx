import React, { ReactNode } from 'react'

export interface IRootProps {
  children?: ReactNode
}
export function Root({ children }: IRootProps) {
  return <div style={{ padding: 24 }}>{children}</div>
}
