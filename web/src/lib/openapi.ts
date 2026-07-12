import { load } from 'js-yaml';

export type HttpMethod = 'get' | 'post' | 'put' | 'delete' | 'patch' | 'head' | 'options' | 'trace';

export interface OpenApiParameter {
  name: string;
  in: 'path' | 'query' | 'header' | 'cookie';
  required?: boolean;
  description?: string;
  schema?: Record<string, unknown>;
}

export interface OpenApiResponse {
  description: string;
  content?: Record<string, { schema?: Record<string, unknown> }>;
}

export interface OpenApiOperation {
  operationId?: string;
  summary?: string;
  description?: string;
  tags?: string[];
  method: HttpMethod;
  path: string;
  parameters?: OpenApiParameter[];
  requestBody?: {
    description?: string;
    required?: boolean;
    content?: Record<string, { schema?: Record<string, unknown> }>;
  };
  responses?: Record<string, OpenApiResponse>;
  security?: Record<string, string[]>[];
  'x-middlewares'?: string[];
}

export interface OpenApiTag {
  name: string;
  description?: string;
}

export interface OpenApiSpec {
  openapi: string;
  info: {
    title: string;
    version: string;
    description?: string;
  };
  servers?: { url: string; description?: string }[];
  tags?: OpenApiTag[];
  paths: Record<string, Record<HttpMethod, OpenApiOperation>>;
  components?: {
    securitySchemes?: Record<string, Record<string, unknown>>;
    schemas?: Record<string, Record<string, unknown>>;
  };
}

export interface GroupedOperations {
  tag: string;
  description?: string;
  operations: OpenApiOperation[];
}

const METHOD_ORDER: HttpMethod[] = ['get', 'post', 'put', 'patch', 'delete', 'head', 'options', 'trace'];

export function methodSortIndex(method: HttpMethod): number {
  return METHOD_ORDER.indexOf(method) ?? METHOD_ORDER.length;
}

export function normalizeSpec(raw: unknown): OpenApiSpec {
  const spec = raw as OpenApiSpec;
  const paths: OpenApiSpec['paths'] = {};

  for (const [path, methods] of Object.entries(spec.paths ?? {})) {
    paths[path] = {} as Record<HttpMethod, OpenApiOperation>;
    for (const [method, operation] of Object.entries(methods ?? {})) {
      if (METHOD_ORDER.includes(method as HttpMethod)) {
        paths[path][method as HttpMethod] = {
          ...operation,
          method: method as HttpMethod,
          path,
        };
      }
    }
  }

  return { ...spec, paths };
}

export function groupOperationsByTag(spec: OpenApiSpec): GroupedOperations[] {
  const map = new Map<string, OpenApiOperation[]>();
  const tagDescriptions = new Map<string, string>();

  for (const tag of spec.tags ?? []) {
    tagDescriptions.set(tag.name, tag.description ?? '');
    if (!map.has(tag.name)) map.set(tag.name, []);
  }

  for (const methods of Object.values(spec.paths)) {
    for (const operation of Object.values(methods)) {
      const tags = operation.tags?.length ? operation.tags : ['untagged'];
      for (const tag of tags) {
        if (!map.has(tag)) map.set(tag, []);
        map.get(tag)!.push(operation);
      }
    }
  }

  // Sort operations within each tag by path then method
  for (const ops of map.values()) {
    ops.sort((a, b) => {
      if (a.path !== b.path) return a.path.localeCompare(b.path);
      return methodSortIndex(a.method) - methodSortIndex(b.method);
    });
  }

  // Preserve tag order from spec; untagged last
  const ordered: GroupedOperations[] = [];
  const seen = new Set<string>();
  for (const tag of spec.tags ?? []) {
    const ops = map.get(tag.name) ?? [];
    ordered.push({ tag: tag.name, description: tagDescriptions.get(tag.name), operations: ops });
    seen.add(tag.name);
  }
  for (const [tag, ops] of map) {
    if (!seen.has(tag)) {
      ordered.push({ tag, operations: ops });
    }
  }

  return ordered.filter((g) => g.operations.length > 0);
}

export function operationSlug(operation: OpenApiOperation): string {
  return `${operation.method}-${operation.path}`;
}

export async function loadOpenApi(url = '/openapi.yaml'): Promise<OpenApiSpec> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`Failed to load OpenAPI spec: ${res.status} ${res.statusText}`);
  const text = await res.text();
  const raw = load(text) as unknown;
  return normalizeSpec(raw);
}

export function getServerUrl(spec: OpenApiSpec): string {
  return spec.servers?.[0]?.url ?? '';
}

export function displayPath(operation: OpenApiOperation): string {
  return operation.path;
}

export function displaySummary(operation: OpenApiOperation): string {
  return operation.summary ?? operation.operationId ?? `${operation.method.toUpperCase()} ${operation.path}`;
}

export function methodColor(method: HttpMethod): string {
  switch (method) {
    case 'get':
      return 'text-get bg-get/10 border-get/20';
    case 'post':
      return 'text-post bg-post/10 border-post/20';
    case 'put':
      return 'text-put bg-put/10 border-put/20';
    case 'patch':
      return 'text-patch bg-patch/10 border-patch/20';
    case 'delete':
      return 'text-delete bg-delete/10 border-delete/20';
    default:
      return 'text-text-muted bg-surface-inset border-border';
  }
}
