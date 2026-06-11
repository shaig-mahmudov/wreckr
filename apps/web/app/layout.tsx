import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Wreckr",
  description: "Production scenario testing for backend systems"
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
