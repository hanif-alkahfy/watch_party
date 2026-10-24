import { useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'

export function UploadForm({ onUpload, busy }) {
  const inputRef = useRef(null)
  const [dragging, setDragging] = useState(false)
  const [progress, setProgress] = useState(null)
  const [error, setError] = useState('')

  const handleFile = async (file) => {
    if (!file) return
    if (!file.name.toLowerCase().endsWith('.mp4')) {
      setError('Only .mp4 files are supported')
      return
    }
    setError('')
    setProgress(0)
    try {
      await onUpload(file, setProgress)
      setProgress(null)
    } catch (err) {
      setError(err.message)
      setProgress(null)
    }
  }

  return (
    <div className="space-y-3">
      <div
        onDragOver={(e) => {
          e.preventDefault()
          setDragging(true)
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault()
          setDragging(false)
          handleFile(e.dataTransfer.files?.[0])
        }}
        onClick={() => inputRef.current?.click()}
        className={`flex min-w-0 cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border border-dashed p-8 text-center transition-colors ${
          dragging ? 'border-primary bg-primary/5' : 'border-border hover:bg-muted/40'
        }`}
      >
        <p className="break-words text-sm font-medium">
          Drop an MP4 file here, or click to browse
        </p>
        <p className="break-words text-xs text-muted-foreground">
          The video is transcoded to HLS in the background
        </p>
        <input
          ref={inputRef}
          id="video-file"
          name="video"
          type="file"
          accept="video/mp4,.mp4"
          className="hidden"
          onChange={(e) => {
            handleFile(e.target.files?.[0])
            e.target.value = ''
          }}
        />
      </div>

      {progress !== null && (
        <div className="space-y-1">
          <Progress value={progress} />
          <p className="text-xs text-muted-foreground">Uploading {progress}%</p>
        </div>
      )}

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Button
        variant="outline"
        className="w-full"
        disabled={busy}
        onClick={(e) => {
          e.stopPropagation()
          inputRef.current?.click()
        }}
      >
        Select video file
      </Button>
    </div>
  )
}
