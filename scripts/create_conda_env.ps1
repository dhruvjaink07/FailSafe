<#
Create or update the Conda environment for FailSafe and print the Python executable path.
Usage (PowerShell):
  .\scripts\create_conda_env.ps1

This script will create the env named 'failsafe-dev' from environment.yml (if missing),
attempt to activate it, and then print the path of the Python executable you should set
in `PYTHON_EXEC` (for example in your environment or in .env).
#>

try {
    $envFile = Join-Path -Path (Get-Location) -ChildPath "environment.yml"
    if (-not (Test-Path $envFile)) {
        Write-Error "environment.yml not found in repository root."
        exit 1
    }

    $conda = Get-Command conda -ErrorAction SilentlyContinue
    if (-not $conda) {
        Write-Error "Conda not found in PATH. Install Anaconda or Miniconda and ensure 'conda' is available.";
        exit 1
    }

    Write-Host "Creating/updating Conda env 'failsafe-dev' from environment.yml..."
    & conda env create -f $envFile --name failsafe-dev 2>$null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Environment may already exist; attempting to update instead..."
        & conda env update -f $envFile --name failsafe-dev
    }

    Write-Host "To use the environment, run: conda activate failsafe-dev"

    # Try to print the python executable path for the user to set PYTHON_EXEC
    $pythonPath = & conda run -n failsafe-dev python -c "import sys;print(sys.executable)" 2>$null
    if ($LASTEXITCODE -eq 0 -and $pythonPath) {
        Write-Host "Detected python executable for 'failsafe-dev':"
        Write-Host $pythonPath
        Write-Host "Set PYTHON_EXEC to that path if Python isn't on your PATH."
    } else {
        Write-Host "Unable to query python path with 'conda run'. After activating the env, run:"
        Write-Host "  python -c \"import sys;print(sys.executable)\""
    }
} catch {
    Write-Error $_.Exception.Message
    exit 1
}
