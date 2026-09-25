import SwaggerUI from 'swagger-ui-react';
import 'swagger-ui-react/swagger-ui.css';
import { useCallback, useState } from 'react';
import { Typography } from '@mui/material';
import { API_BASE } from '../environment/Environment';
import Card from '../ui/Card';
import Inline from '../ui/Inline';
import Stack from '../ui/Stack';
import SwaggerFrame from '../ui/SwaggerFrame';

interface ApiInfo {
  title?: string;
  version?: string;
  description?: string;
}

interface SwaggerSystem {
  specSelectors: { info: () => { toJS: () => ApiInfo } };
}

const plugins = [{ components: { InfoContainer: () => null } }];

export default function ApiReferenceCard() {
  const [info, setInfo] = useState<ApiInfo>({});

  const requestInterceptor = useCallback((req: Record<string, unknown>) => {
    (req as Record<string, unknown>).credentials = 'include';
    return req;
  }, []);

  const handleComplete = useCallback((system: SwaggerSystem) => {
    const { title, version, description } = system.specSelectors.info().toJS();
    setInfo({ title, version, description });
  }, []);

  return (
    <Card title="API Reference">
      <Stack>
        {info.title && (
          <Stack>
            <Inline>
              <Typography variant="sectionTitle">{info.title}</Typography>
              {info.version && (
                <Typography variant="caption" color="text.secondary">
                  Version {info.version}
                </Typography>
              )}
            </Inline>
            {info.description && (
              <Typography variant="bodySmall" color="text.secondary">
                {info.description}
              </Typography>
            )}
          </Stack>
        )}
        <SwaggerFrame>
          <SwaggerUI
            url={`${API_BASE}/openapi.yaml`}
            requestInterceptor={requestInterceptor}
            onComplete={handleComplete}
            plugins={plugins}
          />
        </SwaggerFrame>
      </Stack>
    </Card>
  );
}
