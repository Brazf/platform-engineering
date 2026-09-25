"use client";

const EYE = "M12 5c-5.2 0-9.3 4.2-10.8 6.1a1.4 1.4 0 0 0 0 1.8C2.7 14.8 6.8 19 12 19s9.3-4.2 10.8-6.1a1.4 1.4 0 0 0 0-1.8C21.3 9.2 17.2 5 12 5Zm0 11a5 5 0 1 1 0-10 5 5 0 0 1 0 10Zm0-2.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Z";
const SLASH = "M3.3 2 22 20.7 20.7 22 2 3.3 3.3 2Z";

/** Botão do olho dentro do campo de senha: alterna o input entre "password" e "text". */
export function PasswordToggle({ visible, onToggle }: { visible: boolean; onToggle: () => void }) {
  const label = visible ? "Ocultar senha" : "Mostrar senha";
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-label={label}
      aria-pressed={visible}
      title={label}
      className="flex h-8 w-8 items-center justify-center text-[#1351B4]"
    >
      <svg viewBox="0 0 24 24" className="h-4 w-4 fill-current" aria-hidden>
        <path d={EYE} />
        {visible && <path d={SLASH} />}
      </svg>
    </button>
  );
}
