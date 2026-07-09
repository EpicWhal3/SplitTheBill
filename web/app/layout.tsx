import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "SplitTheBill",
  description: "Split restaurant bills with friends",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ru">
      <body>{children}</body>
    </html>
  );
}
