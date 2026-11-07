import { describe, expect, it } from 'vitest';

import { publicationURL } from '@/lib/publicationUrl';

// A copied link is pasted into external clients; a bare path is useless there.
describe('publicationURL', () => {
  const base = {
    token: 'TOK',
    subClashURI: '',
    subURI: '',
    subPort: 2096,
    panelURL: 'http://203.0.113.7:5566/',
  };

  it('always returns an absolute URL, never a bare path', () => {
    const url = publicationURL(base);
    expect(url.startsWith('/')).toBe(false);
    expect(url).toBe('http://203.0.113.7:2096/subconv/TOK');
  });

  it('prefers the configured clash URI origin when set (reverse proxy)', () => {
    const url = publicationURL({
      ...base,
      subClashURI: 'https://sub.example.com/q7zt3mxk2rv8w4pn/',
    });
    expect(url).toBe('https://sub.example.com/subconv/TOK');
  });

  it('falls back to the generic sub URI origin before deriving from the panel host', () => {
    const url = publicationURL({ ...base, subURI: 'https://dl.example.com/k9wd3qxn8mtv2rhb/' });
    expect(url).toBe('https://dl.example.com/subconv/TOK');
  });

  it('keeps an explicit port but drops the scheme default one', () => {
    expect(publicationURL(base)).toContain(':2096/');
    expect(publicationURL({ ...base, subPort: 80 })).toBe('http://203.0.113.7/subconv/TOK');
    expect(publicationURL({ ...base, panelURL: 'https://h.example/', subPort: 443 })).toBe(
      'https://h.example/subconv/TOK',
    );
  });
});
