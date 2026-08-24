"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import LogoIcon from "./LogoIcon";

const NAV_LINKS = [
  { href: "/", label: "主页" },
  { href: "/manager", label: "Manager" },
  { href: "/service", label: "Service" },
];

export default function Nav() {
  const [menuOpen, setMenuOpen] = useState(false);
  const pathname = usePathname();

  function closeMenu() {
    setMenuOpen(false);
  }

  return (
    <header className="nav">
      <div className="container nav-inner">
        <Link className="logo" href="/" onClick={closeMenu}>
          <LogoIcon size={30} />
          <span className="logo-text">LianT · 联T</span>
        </Link>

        <nav className="nav-links" aria-label="站点导航">
          {NAV_LINKS.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              className={pathname === link.href ? "router-link-active" : ""}
            >
              {link.label}
            </Link>
          ))}
        </nav>

        <button
          className="menu-btn"
          type="button"
          aria-label="打开菜单"
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen((v) => !v)}
        >
          <span className="bar" />
          <span className="bar" />
          <span className="bar" />
        </button>
      </div>

      {menuOpen && (
        <div className="menu-popover" onClick={(e) => e.target === e.currentTarget && closeMenu()}>
          {NAV_LINKS.map((link) => (
            <Link
              key={link.href}
              href={link.href}
              onClick={closeMenu}
              className={pathname === link.href ? "router-link-active" : ""}
            >
              {link.label}
            </Link>
          ))}
        </div>
      )}
    </header>
  );
}
