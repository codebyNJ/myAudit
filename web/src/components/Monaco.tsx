import { useEffect, useRef } from 'react'
import Editor, { loader, type OnMount } from '@monaco-editor/react'
import * as MonacoNS from 'monaco-editor'
import { detectLanguage } from './detectLanguage'
export { detectLanguage }


;(self as unknown as { MonacoEnvironment: MonacoNS.Environment }).MonacoEnvironment = {
  getWorker() {
    return new Worker(URL.createObjectURL(new Blob(['self.onmessage=()=>{}'], { type: 'application/javascript' })))
  },
}
loader.config({ monaco: MonacoNS })


const THEME: MonacoNS.editor.IStandaloneThemeData = {
  base: 'vs-dark',
  inherit: true,
  rules: [
    { token: '', foreground: 'e4e4e7' },
    { token: 'comment', foreground: '6b7280', fontStyle: 'italic' },
    { token: 'keyword', foreground: 'c586c0' },
    { token: 'string', foreground: 'ce9178' },
    { token: 'number', foreground: 'b5cea8' },
    { token: 'type', foreground: '4ec9b0' },
    { token: 'function', foreground: 'dcdcaa' },
    { token: 'variable', foreground: '9cdcfe' },
    { token: 'operator', foreground: 'd4d4d4' },
    { token: 'delimiter', foreground: 'd4d4d4' },
    { token: 'tag', foreground: '569cd6' },
    { token: 'attribute.name', foreground: '9cdcfe' },
    { token: 'attribute.value', foreground: 'ce9178' },

    { token: 'tag.css', foreground: 'd7ba7d' },
    { token: 'attribute.name.css', foreground: '9cdcfe' },
    { token: 'attribute.value.css', foreground: 'ce9178' },
    { token: 'attribute.value.number.css', foreground: 'b5cea8' },
    { token: 'attribute.value.unit.css', foreground: 'b5cea8' },
    { token: 'attribute.value.hex.css', foreground: 'ce9178' },
    { token: 'keyword.css', foreground: 'c586c0' },
    { token: 'string.css', foreground: 'ce9178' },
    { token: 'comment.css', foreground: '6b7280', fontStyle: 'italic' },
    { token: 'delimiter.css', foreground: 'd4d4d4' },
    { token: 'delimiter.bracket.css', foreground: 'ffd700' },
    { token: 'meta.scss', foreground: 'c586c0' },
    { token: 'variable.scss', foreground: '9cdcfe' },
    { token: 'keyword.scss', foreground: 'c586c0' },
    { token: 'key', foreground: '9cdcfe' },
    { token: 'string.key.json', foreground: '9cdcfe' },
    { token: 'string.value.json', foreground: 'ce9178' },
  ],
  colors: {
    'editor.background': '#0c0c0e',
    'editor.foreground': '#d4d4d4',
    'editorLineNumber.foreground': '#5a5a5a',
    'editorLineNumber.activeForeground': '#c6c6c6',
    'editor.lineHighlightBackground': '#18181b',
    'editorCursor.foreground': '#aeafad',
    'editor.selectionBackground': '#264f78',
    'editor.inactiveSelectionBackground': '#3a3d41',
    'editorIndentGuide.background1': '#3f3f46',
    'editorIndentGuide.activeBackground1': '#71717a',
    'editorWhitespace.foreground': '#3f3f4688',
    'editorWidget.background': '#1e1e1e',
    'editorWidget.border': '#454545',
    'input.background': '#1e1e1e',
    'scrollbarSlider.background': '#ffffff14',
    'scrollbarSlider.hoverBackground': '#ffffff24',
    'scrollbarSlider.activeBackground': '#ffffff34',
  },
}

let themeReady = false
function ensureTheme(m: typeof MonacoNS) {
  if (themeReady) return
  m.editor.defineTheme('cursor-dark', THEME)


  type Defaults = { setDiagnosticsOptions?: (o: object) => void }
  const ts = (m.languages as unknown as {
    typescript?: { typescriptDefaults?: Defaults; javascriptDefaults?: Defaults }
  }).typescript
  const off = { noSemanticValidation: true, noSyntaxValidation: true }
  ts?.typescriptDefaults?.setDiagnosticsOptions?.(off)
  ts?.javascriptDefaults?.setDiagnosticsOptions?.(off)
  themeReady = true
}

export type MonoProps = {
  path: string
  value: string
  readOnly?: boolean
  wordWrap?: boolean
  minimap?: boolean
  onChange?: (v: string) => void
  onCursor?: (line: number, col: number) => void
  onSave?: () => void
  findTrigger?: number
}

export function Mono({
  path, value, readOnly = false, wordWrap = true, minimap = false,
  onChange, onCursor, onSave, findTrigger = 0,
}: MonoProps) {
  const edRef = useRef<MonacoNS.editor.IStandaloneCodeEditor | null>(null)
  const saveRef = useRef(onSave)
  saveRef.current = onSave

  useEffect(() => {
    const ed = edRef.current
    if (!ed) return
    ed.updateOptions({
      readOnly,
      wordWrap: wordWrap ? 'on' : 'off',
      minimap: { enabled: minimap },
    })
  }, [readOnly, wordWrap, minimap])

  useEffect(() => {
    if (!findTrigger) return
    void edRef.current?.getAction('actions.find')?.run()
  }, [findTrigger])

  const onMount: OnMount = (editor, m) => {
    edRef.current = editor
    ensureTheme(m)
    m.editor.setTheme('cursor-dark')
    editor.addCommand(m.KeyMod.CtrlCmd | m.KeyCode.KeyS, () => saveRef.current?.())
    editor.onDidChangeCursorPosition((e) => {
      onCursor?.(e.position.lineNumber, e.position.column)
    })
    const model = editor.getModel()
    if (model) {

      model.detectIndentation(true, 2)
    }
    editor.focus()
  }

  return (
    <Editor
      key={path}
      theme="cursor-dark"
      language={detectLanguage(path)}
      path={path}
      value={value}
      onChange={(v) => onChange?.(v ?? '')}
      onMount={onMount}
      loading={<div className="empty-mid" style={{ position: 'static', paddingTop: 60 }}><div className="spin" /></div>}
      height="100%"
      options={{
        readOnly,
        fontFamily: "'Geist Mono', 'JetBrains Mono', 'SF Mono', Menlo, Consolas, monospace",
        fontSize: 15,
        lineHeight: 22,
        letterSpacing: 0,
        minimap: { enabled: minimap, scale: 1, showSlider: 'mouseover' },
        scrollBeyondLastLine: false,
        wordWrap: wordWrap ? 'on' : 'off',
        tabSize: 2,
        insertSpaces: true,
        detectIndentation: true,
        automaticLayout: true,
        padding: { top: 10, bottom: 10 },
        renderLineHighlight: 'all',
        matchBrackets: 'always',
        occurrencesHighlight: 'singleFile',
        selectionHighlight: true,
        bracketPairColorization: { enabled: true },
        guides: { indentation: true, bracketPairs: true, highlightActiveIndentation: true },
        contextmenu: true,
        folding: true,
        foldingHighlight: true,
        glyphMargin: true,
        lineNumbers: 'on',
        lineNumbersMinChars: 3,
        lineDecorationsWidth: 12,
        renderWhitespace: 'boundary',
        smoothScrolling: true,
        cursorBlinking: 'smooth',
        cursorSmoothCaretAnimation: 'on',
        find: { addExtraSpaceOnTop: false, autoFindInSelection: 'never' },
        scrollbar: { verticalScrollbarSize: 10, horizontalScrollbarSize: 10, useShadows: false },
        quickSuggestions: !readOnly,
        suggestOnTriggerCharacters: !readOnly,
        formatOnPaste: true,
        links: true,
        mouseWheelZoom: true,
      }}
    />
  )
}
