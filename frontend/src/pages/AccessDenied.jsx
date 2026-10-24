import { ShieldAlert } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

export function AccessDeniedPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/30 p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <div className="flex items-center gap-3">
            <div className="rounded-full bg-destructive/15 p-2 text-destructive">
              <ShieldAlert className="h-5 w-5" />
            </div>
            <CardTitle>Access Denied</CardTitle>
          </div>
          <CardDescription>
            This page is only reachable from this machine or the local network. You are
            connecting from{' '}
            <span className="font-mono text-xs">{window.location.hostname}</span>.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            The watch page stays available for viewers with an invite link.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
