import { Navigate } from 'react-router';
import { useAuth } from '../providers/AuthContext.tsx';
import { hasPerm } from '../tools/Utils.ts';

interface RequireAuthProps {
  permission?: string;
  children: React.ReactElement;
}

export default function RequireAuth({ permission, children }: RequireAuthProps) {
  const { user } = useAuth();
  if (user === undefined) return null;
  if (user === null) return <Navigate to="/login" replace />;
  if (permission !== undefined && !hasPerm(user, permission)) return null;
  return children;
}
