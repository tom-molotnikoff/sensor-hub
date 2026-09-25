import { createElement, type ComponentType, type ReactNode } from 'react';

interface SwaggerSystemProps {
  getComponent: (name: string, container?: boolean) => ComponentType<Record<string, unknown>>;
  specSelectors: {
    version: () => string | undefined;
    url: () => string | undefined;
    basePath: () => string | undefined;
    host: () => string | undefined;
    selectInfoTitleField: () => string | undefined;
    selectInfoSummaryField: () => string | undefined;
    selectInfoDescriptionField: () => string | undefined;
    selectInfoTermsOfServiceUrl: () => string | undefined;
    selectExternalDocsUrl: () => string | undefined;
    selectExternalDocsDescriptionField: () => string | undefined;
    contact: () => { size: number };
    license: () => { size: number };
  };
  fn: { sanitizeUrl: (url: string) => string };
}

export default function ApiInfo({ getComponent, specSelectors, fn }: SwaggerSystemProps) {
  const part = (name: string, props: Record<string, unknown> = {}, children?: ReactNode, container = false) =>
    createElement(getComponent(name, container), props, children);
  const version = specSelectors.version();
  const url = specSelectors.url();
  const basePath = specSelectors.basePath();
  const host = specSelectors.host();
  const summary = specSelectors.selectInfoSummaryField();
  const termsOfService = specSelectors.selectInfoTermsOfServiceUrl();
  const externalDocsUrl = specSelectors.selectExternalDocsUrl();

  return (
    <div className="info">
      <hgroup className="main">
        <h3 className="title">
          {specSelectors.selectInfoTitleField()}
          <span>
            {version && part('VersionStamp', { version })}
            {part('OpenAPIVersion', { oasVersion: '3.1' })}
          </span>
        </h3>
        {(host || basePath) && part('InfoBasePath', { host, basePath })}
        {url && part('InfoUrl', { getComponent, url })}
      </hgroup>
      {summary && <p className="info__summary">{summary}</p>}
      <div className="info__description description">
        {part('Markdown', { source: specSelectors.selectInfoDescriptionField() }, undefined, true)}
      </div>
      {termsOfService && (
        <div className="info__tos">
          {part('Link', { target: '_blank', href: fn.sanitizeUrl(termsOfService) }, 'Terms of service')}
        </div>
      )}
      {specSelectors.contact().size > 0 && part('Contact', {}, undefined, true)}
      {specSelectors.license().size > 0 && part('License', {}, undefined, true)}
      {externalDocsUrl &&
        part(
          'Link',
          { className: 'info__extdocs', target: '_blank', href: fn.sanitizeUrl(externalDocsUrl) },
          specSelectors.selectExternalDocsDescriptionField() || externalDocsUrl,
        )}
      {part('JsonSchemaDialect', {}, undefined, true)}
    </div>
  );
}
