import SwaggerUI from 'swagger-ui-react';
import 'swagger-ui-react/swagger-ui.css';
import { Box } from '@mui/material';
import { TypographyH2 } from '../tools/Typography';
import { API_BASE } from '../environment/Environment';
import { useCallback } from 'react';
import { useIsDark } from '../ui/theme/useIsDark';

export default function ApiReferenceCard() {
  const isDark = useIsDark();

  const requestInterceptor = useCallback((req: Record<string, unknown>) => {
    (req as Record<string, unknown>).credentials = 'include';
    return req;
  }, []);

  return (
    <Box sx={{
      mt: 3,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      width: '100%',
    }}>
      <TypographyH2>API Reference</TypographyH2>
      <Box
        className={isDark ? 'swagger-dark' : undefined}
        sx={{
          width: '100%',
          borderRadius: 1,
          overflow: 'hidden',

          // ── Light-mode overrides ──
          '.swagger-ui .topbar': {
            bgcolor: 'primary.main',
            padding: '8px 0',
          },
          '.swagger-ui .topbar .download-url-wrapper .select-label select': {
            borderColor: 'rgba(255,255,255,0.5)',
          },
          '.swagger-ui .info .title': {
            color: 'text.primary',
          },
          '.swagger-ui .info p, .swagger-ui .info li': {
            color: 'text.secondary',
          },
          '.swagger-ui .opblock-tag': {
            borderBottom: '1px solid',
            borderColor: 'divider',
          },
          '.swagger-ui .scheme-container': {
            bgcolor: 'background.paper',
            boxShadow: 'none',
            borderBottom: '1px solid',
            borderColor: 'divider',
          },

          // ── Dark-mode overrides ──
          '&.swagger-dark': {
            bgcolor: '#1A1A1A',
            color: '#E8E8E8',
          },
          '&.swagger-dark .swagger-ui': {
            color: '#E8E8E8',
          },
          '&.swagger-dark .swagger-ui .info .title': {
            color: '#fff',
          },
          '&.swagger-dark .swagger-ui .info p, &.swagger-dark .swagger-ui .info li, &.swagger-dark .swagger-ui .info table td': {
            color: '#ddd',
          },
          '&.swagger-dark .swagger-ui .opblock-tag': {
            color: '#f0f0f0',
            borderColor: '#444',
          },
          '&.swagger-dark .swagger-ui .opblock-tag small': {
            color: '#bbb',
          },
          '&.swagger-dark .swagger-ui .opblock-tag:hover': {
            bgcolor: '#2a2a2a',
          },
          '&.swagger-dark .swagger-ui .opblock .opblock-summary': {
            borderColor: '#444',
          },
          '&.swagger-dark .swagger-ui .opblock .opblock-summary-description': {
            color: '#ddd',
          },
          '&.swagger-dark .swagger-ui .opblock .opblock-section-header': {
            bgcolor: '#2a2a2a',
          },
          '&.swagger-dark .swagger-ui .opblock .opblock-section-header h4': {
            color: '#f0f0f0',
          },
          '&.swagger-dark .opblock-summary-path': {
            color: '#88b8f0',
          },
          '&.swagger-dark .swagger-ui .opblock-description-wrapper p, &.swagger-dark .swagger-ui .opblock-external-docs-wrapper p': {
            color: '#ddd',
          },
          '&.swagger-dark .swagger-ui table thead tr th, &.swagger-dark .swagger-ui table thead tr td': {
            color: '#ddd',
            borderColor: '#444',
          },
          '&.swagger-dark .swagger-ui .parameter__name': {
            color: '#f0f0f0',
          },
          '&.swagger-dark .swagger-ui .parameter__type': {
            color: '#bbb',
          },
          '&.swagger-dark .swagger-ui .parameter__in': {
            color: '#999',
          },
          '&.swagger-dark .swagger-ui table tbody tr td': {
            color: '#ddd',
            borderColor: '#333',
          },
          '&.swagger-dark .swagger-ui .scheme-container': {
            bgcolor: '#1A1A1A',
            borderColor: '#444',
          },
          '&.swagger-dark .swagger-ui .scheme-container .schemes > label': {
            color: '#ddd',
          },
          '&.swagger-dark .swagger-ui select': {
            bgcolor: '#2a2a2a',
            color: '#f0f0f0',
            borderColor: '#555',
          },
          '&.swagger-dark .swagger-ui input[type=text]': {
            bgcolor: '#2a2a2a',
            color: '#f0f0f0',
            borderColor: '#555',
          },
          '&.swagger-dark .swagger-ui textarea': {
            bgcolor: '#2a2a2a',
            color: '#f0f0f0',
            borderColor: '#555',
          },
          '&.swagger-dark .swagger-ui .model-container': {
            bgcolor: '#2a2a2a',
          },
          '&.swagger-dark .swagger-ui .model': {
            color: '#ddd',
          },
          '&.swagger-dark .swagger-ui .model .prop-type': {
            color: '#88b8f0',
          },
          '&.swagger-dark .swagger-ui .model-title': {
            color: '#f0f0f0',
          },
          '&.swagger-dark .swagger-ui section.models': {
            borderColor: '#444',
          },
          '&.swagger-dark .swagger-ui section.models h4': {
            color: '#f0f0f0',
            borderColor: '#444',
          },
          '&.swagger-dark .swagger-ui section.models .model-box': {
            bgcolor: '#242424',
          },
          '&.swagger-dark .swagger-ui .response-col_status': {
            color: '#f0f0f0',
          },
          '&.swagger-dark .swagger-ui .response-col_description__inner p': {
            color: '#ddd',
          },
          '&.swagger-dark .swagger-ui .responses-inner h4, &.swagger-dark .swagger-ui .responses-inner h5': {
            color: '#f0f0f0',
          },
          '&.swagger-dark .swagger-ui .opblock-body pre.microlight': {
            bgcolor: '#1a1a1a',
            color: '#e0e0e0',
            borderColor: '#444',
          },
          '&.swagger-dark .swagger-ui .btn': {
            color: '#e0e0e0',
            borderColor: '#555',
          },
          '&.swagger-dark .swagger-ui .btn.execute': {
            color: '#fff',
          },
          '&.swagger-dark .models': {
            bgcolor: '#2a2a2a',
          },
          '&.swagger-dark .json-schema-2020-12': {
            bgcolor: '#2a2a2a',
          },
          '&.swagger-dark .json-schema-2020-12-accordion': {
            bgcolor: '#2a2a2a',
          },
          '&.swagger-dark .json-schema-2020-12__title': {
            color: '#88b8f0',
          }
        }}
      >
        <SwaggerUI
          url={`${API_BASE}/openapi.yaml`}
          requestInterceptor={requestInterceptor}
        />
      </Box>
    </Box>
  );
}
