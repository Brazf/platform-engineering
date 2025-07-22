'use client'

import { useRouter } from 'next/navigation'
import Image from 'next/image';

export default function LoginPage() {
  const router = useRouter()

  function handleLogin(event) {
    event.preventDefault()
    // colocar validação de login
    router.push('/dashboard')
  }

  return (
    <main className="flex justify-between">
      <div className="flex justify-end items-center w-1/2 h-screen bg-red-50">
        <Image className="bg-red-400" src="/images/imagem_principal.svg" alt="Imagem principal" width={522} height={410} />  
      </div>
    
      <div className="w-1/2">
        <form onSubmit={handleLogin}>
          <input type="text" placeholder="Usuário" /><br/>
          <input type="password" placeholder="Senha" /><br/>
          <button type="submit">Pagina</button>
        </form>
      </div>
    
    </main>
  )
}

// <div class="w-1/2 h-screen bg-blue-200 flex items-center justify-center text-xl font-bold">
//  Quadro de 50% Largura e 100% Altura da Viewport
//</div>
