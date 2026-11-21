/** Jest config: TypeScript via ts-jest, DOM via jsdom. */
module.exports = {
  testEnvironment: 'jsdom',
  roots: ['<rootDir>/src'],
  setupFilesAfterEnv: ['<rootDir>/src/setupTests.ts'],
  transform: {
    '^.+\\.tsx?$': ['ts-jest', { tsconfig: { jsx: 'react-jsx', module: 'commonjs', esModuleInterop: true, moduleResolution: 'node', types: ['jest', '@testing-library/jest-dom'] } }],
  },
  moduleNameMapper: {
    '\\.css$': 'identity-obj-proxy',
    '^axios$': 'axios/dist/node/axios.cjs',
  },
};
