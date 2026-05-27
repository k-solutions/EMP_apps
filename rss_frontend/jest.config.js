export default {
  testEnvironment: 'jsdom',
  transform: {
    '^.+\\.(js|jsx|ts|tsx)$': 'babel-jest',
  },
  setupFilesAfterEnv: ['<rootDir>/app/frontend/setupTests.js'],
  testMatch: [
    '**/app/frontend/**/*.test.(js|jsx|ts|tsx)'
  ]
}
