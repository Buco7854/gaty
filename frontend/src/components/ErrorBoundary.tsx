import { Component, type ReactNode } from 'react'
import { Container, Title, Text, Button, Stack, Paper } from '@mantine/core'
import { AlertTriangle } from 'lucide-react'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error('ErrorBoundary caught:', error, info.componentStack)
  }

  render() {
    if (this.state.hasError) {
      return (
        <Container size="sm" py="xl">
          <Paper p="xl" withBorder>
            <Stack align="center" gap="md">
              <AlertTriangle size={48} color="var(--mantine-color-red-6)" />
              <Title order={3}>Something went wrong</Title>
              <Text c="dimmed" ta="center">
                An unexpected error occurred. Please try reloading the page.
              </Text>
              {this.state.error && (
                <Text size="xs" c="dimmed" ff="monospace" ta="center">
                  {this.state.error.message}
                </Text>
              )}
              <Button
                variant="light"
                onClick={() => {
                  this.setState({ hasError: false, error: null })
                  window.location.reload()
                }}
              >
                Reload page
              </Button>
            </Stack>
          </Paper>
        </Container>
      )
    }

    return this.props.children
  }
}
