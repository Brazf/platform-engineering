import '@/app/globals.css';


export default function RootLayout({ children }) {
  return (
    <html lang="pt-BR">
      <body className="bg-[FFFFFF]">
        {children}
      </body>
    </html>
  )
}
