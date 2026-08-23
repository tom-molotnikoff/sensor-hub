import React, { useEffect, useState } from 'react';
import { apiClient } from '../gen/client';
import { setCsrfToken } from '../api/Csrf';
import { AuthContext, type AuthUser } from './AuthContext.tsx';

export default function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | undefined>(undefined);

  const refresh = () =>
    apiClient.GET('/auth/me')
      .then(({ data }) => {
        if (data?.csrf_token) setCsrfToken(data.csrf_token);
        setUser((data?.user as AuthUser) ?? null);
      })
      .catch(() => setUser(null));

  useEffect(() => { void refresh(); }, []);

  return (
    <AuthContext.Provider value={{ user, refresh }}>
      {children}
    </AuthContext.Provider>
  );
}
