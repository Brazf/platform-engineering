'use client'

const Button = ({
  children, // O texto ou ícone dentro do botão
  onClick,  // Função a ser executada ao clicar
  variant = 'primary', // Estilo do botão: 'primary', 'secondary', 'destructive', etc.
  size = 'md', // Tamanho do botão: 'sm', 'md', 'lg'
  type = 'button', // Tipo do botão: 'button', 'submit', 'reset'
  className = '', // Classes adicionais para personalização extra
  ...props // Outras props HTML nativas (disabled, aria-label, etc.)
}) => {
  // Define as classes base para todos os botões
  const baseClasses = 'font-semibold py-2 px-4 rounded-lg transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-opacity-75';

  // Define as classes de variante
  const variantClasses = {
    primary: 'bg-blue-600 hover:bg-blue-700 text-white focus:ring-blue-500',
    secondary: 'bg-gray-200 hover:bg-gray-300 text-gray-800 focus:ring-gray-400',
    destructive: 'bg-red-600 hover:bg-red-700 text-white focus:ring-red-500',
    outline: 'border border-blue-600 text-blue-600 hover:bg-blue-50 focus:ring-blue-500',
    link: 'text-blue-600 hover:underline',
    // Adicione mais variantes conforme necessário
  };

  // Define as classes de tamanho
  const sizeClasses = {
    sm: 'text-sm py-1 px-3',
    md: 'text-base py-2 px-4',
    lg: 'text-lg py-3 px-6',
    // Adicione mais tamanhos conforme necessário
  };

  // Combina todas as classes
  const allClasses = `${baseClasses} ${variantClasses[variant]} ${sizeClasses[size]} ${className}`;

  return (
    <button
      type={type}
      onClick={onClick}
      className={allClasses}
      {...props}
    >
      {children}
    </button>
  );
};

export default Button;