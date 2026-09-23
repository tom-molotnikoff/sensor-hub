import type { CSSProperties } from "react";

interface TypographyProps {
  children: React.ReactNode;
  testid?: string;
  changes?: CSSProperties;
}

export function TypographyH2({ children, testid, changes }: TypographyProps) {
  return (
    <h2 data-testid={testid} style={{ marginBottom: 8, ...changes }}>
      {children}
    </h2>
  );
}
export function TypographyH3({ children, testid, changes }: TypographyProps) {
  return (
    <h3 data-testid={testid} style={changes}>
      {children}
    </h3>
  );
}
