/** A Material Symbols glyph (the icon font Minzar used). */
export const Icon = ({
  name,
  filled,
  size,
}: {
  name: string;
  filled?: boolean;
  size?: number;
}) => (
  <span
    className={`material-symbols-outlined${filled ? ' mz-filled' : ''}`}
    style={size ? { fontSize: size } : undefined}
    aria-hidden="true"
  >
    {name}
  </span>
);
