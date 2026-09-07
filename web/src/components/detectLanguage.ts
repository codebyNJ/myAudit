const EXT_LANG_MAP: Record<string, string> = {
  ts: 'typescript', tsx: 'typescript', js: 'javascript', jsx: 'javascript',
  mjs: 'javascript', cjs: 'javascript', py: 'python', pyw: 'python',
  go: 'go', rs: 'rust', rb: 'ruby', php: 'php', java: 'java',
  c: 'c', h: 'c', cpp: 'cpp', cc: 'cpp', cxx: 'cpp', hpp: 'cpp',
  cs: 'csharp', swift: 'swift', kt: 'kotlin', kts: 'kotlin', scala: 'scala',
  dart: 'dart', css: 'css', scss: 'scss', sass: 'scss', less: 'less',
  html: 'html', htm: 'html', xml: 'xml', svg: 'xml', vue: 'html', svelte: 'html',
  json: 'json', jsonc: 'json', yaml: 'yaml', yml: 'yaml', toml: 'ini', ini: 'ini',
  env: 'shell', md: 'markdown', markdown: 'markdown', mdx: 'markdown',
  sh: 'shell', bash: 'shell', zsh: 'shell', fish: 'shell', sql: 'sql',
  graphql: 'graphql', gql: 'graphql', lua: 'lua', r: 'r', proto: 'proto',
}

export function detectLanguage(path: string): string {
  const base = path.toLowerCase().split('/').pop() || ''
  if (base === 'dockerfile' || base.startsWith('dockerfile.')) return 'dockerfile'
  if (base === 'makefile' || base === 'gnumakefile') return 'makefile'
  if (base === '.gitignore' || base === '.npmignore' || base === '.dockerignore') return 'shell'
  const ext = base.includes('.') ? base.split('.').pop()! : ''
  return EXT_LANG_MAP[ext] || 'plaintext'
}
