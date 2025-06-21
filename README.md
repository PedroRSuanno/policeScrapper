# Police Reservation Scraper

A web scraper for checking appointment availability at Tokyo Metropolitan Police Driver's License Centers. This scraper runs automatically via GitHub Actions.

## Features

- Checks for available slots every 15 minutes
- Sends notifications via LINE when slots are found
- Runs completely in GitHub Actions
- Includes security checks and dependency updates

## Setup

### 1. GitHub Repository Setup

1. Fork or clone this repository
2. Go to Settings > Secrets and variables > Actions
3. Add the following repository secrets:
   - `LINE_CHANNEL_TOKEN`: Your LINE Notify token
   - `LINE_USER_ID`: Your LINE user ID

### 2. Enable GitHub Actions

1. Go to Actions tab in your repository
2. Enable workflows if not already enabled
3. The scraper will automatically run every 15 minutes

## Local Development

For testing locally:

1. Copy configuration template:

   ```bash
   cp config.example.sh config.local.sh
   ```

2. Edit `config.local.sh` with your LINE credentials

3. Run the scraper:
   ```bash
   go run cmd/scraper/main.go
   ```

## Configuration

The scraper supports two modes:

- Test mode: `go run cmd/scraper/main.go test`
- Real mode: `go run cmd/scraper/main.go`

Additional flags:

- `--no-notify`: Run without sending LINE notifications
- `notify-test`: Test LINE notification setup

## Logs

- Logs are available in GitHub Actions run history
- Failed runs upload logs as artifacts for debugging
- Local runs create logs in the `logs/` directory

## Security

- No sensitive data is stored in the repository
- All credentials are stored in GitHub Secrets
- Regular security scans via GitHub Actions
- Dependabot keeps dependencies up to date

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

[Your License Here]
