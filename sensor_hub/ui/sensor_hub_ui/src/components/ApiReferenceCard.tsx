import SwaggerUI from 'swagger-ui-react';
import 'swagger-ui-react/swagger-ui.css';
import { useCallback } from 'react';
import { API_BASE } from '../environment/Environment';
import Card from '../ui/Card';
import SwaggerFrame from '../ui/SwaggerFrame';
import ApiInfo from './ApiInfo';

const plugins = [{ components: { OAS31Info: ApiInfo } }];

export default function ApiReferenceCard() {
  const requestInterceptor = useCallback((req: Record<string, unknown>) => {
    (req as Record<string, unknown>).credentials = 'include';
    return req;
  }, []);

  return (
    <Card title="API Reference">
      <SwaggerFrame>
        <SwaggerUI
          url={`${API_BASE}/openapi.yaml`}
          requestInterceptor={requestInterceptor}
          plugins={plugins}
        />
      </SwaggerFrame>
    </Card>
  );
}
