'use client'

import Link from 'next/link'

export default function Navbar() {
  return (
    <nav style={{ marginBottom: 20 }}>
      <Link href="/" style={{ marginRight: 10 }}>Login</Link>
      <Link href="/dashboard" style={{ marginRight: 10 }}>Dashboard</Link>
      <Link href="/profile" style={{ marginRight: 10 }}>Perfil</Link>
      <Link href="/form">Formulário</Link>
    </nav>
  )
}
