import { LOGO_PATHS } from "@/lib/logo";

export default function LogoIcon({ size = 30 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 2000 2000"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      {LOGO_PATHS.map((p, i) => (
        <path key={i} d={p.d} fill={p.fill} />
      ))}
    </svg>
  );
}
