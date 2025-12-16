#!/bin/bash

# KBGO Frontend Quick Start Script

echo "🚀 KBGO Frontend Quick Start"
echo "=============================="
echo ""

# Check if node is installed
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install Node.js 18+ first."
    exit 1
fi

echo "✅ Node.js version: $(node -v)"
echo ""

# Check if npm is installed
if ! command -v npm &> /dev/null; then
    echo "❌ npm is not installed."
    exit 1
fi

echo "✅ npm version: $(npm -v)"
echo ""

# Check if node_modules exists
if [ ! -d "node_modules" ]; then
    echo "📦 Installing dependencies..."
    npm install
    echo ""
fi

# Check if backend is running
echo "🔍 Checking backend server..."
if curl -s http://localhost:8000/v1/kb > /dev/null 2>&1; then
    echo "✅ Backend is running on http://localhost:8000"
else
    echo "⚠️  Backend is not running on http://localhost:8000"
    echo "   Please start the backend server first:"
    echo "   cd .. && go run main.go"
fi
echo ""

# Start dev server
echo "🎯 Starting development server..."
echo "   Frontend will be available at: http://localhost:3000"
echo ""
npm run dev