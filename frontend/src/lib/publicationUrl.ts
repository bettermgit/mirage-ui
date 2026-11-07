export interface PublicationURLOptions {
  token: string;
  subClashURI: string;
  subURI: string;
  subPort: number;
  panelURL?: string;
}

function originFrom(uri: string): string {
  try {
    const u = new URL(uri);
    if (u.protocol === 'http:' || u.protocol === 'https:') return u.origin;
  } catch {
    /* not an absolute URI */
  }
  return '';
}

// Publications are served by the subscription server. Prefer configured
// origins (reverse-proxy setups); otherwise derive scheme from the panel and
// host+port from the sub-server settings so the copy is always absolute.
export function publicationURL(o: PublicationURLOptions): string {
  const configured = originFrom(o.subClashURI) || originFrom(o.subURI);
  if (configured) return `${configured}/subconv/${o.token}`;

  let scheme = 'http:';
  let host = 'localhost';
  try {
    const panel = new URL(o.panelURL ?? window.location.href);
    scheme = panel.protocol;
    host = panel.hostname;
  } catch {
    /* keep defaults; result still absolute */
  }
  const port = o.subPort > 0 ? o.subPort : 2096;
  const dropPort = (scheme === 'http:' && port === 80) || (scheme === 'https:' && port === 443);
  const bracketed = host.includes(':') && !host.startsWith('[') ? `[${host}]` : host;
  const hostPart = dropPort ? bracketed : `${bracketed}:${port}`;
  return `${scheme}//${hostPart}/subconv/${o.token}`;
}
