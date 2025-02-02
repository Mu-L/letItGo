module.exports = {
    apps: [
        {
            name: 'webhook',
            script: 'go',
            args: 'run main.go',
            cwd: './webhook',
            interpreter: 'none'
        },
        {
            name: 'producer',
            script: 'go',
            args: 'run main.go',
            cwd: './producer',
            interpreter: 'none'
        },
        {
            name: 'consumer',
            script: 'go',
            args: 'run main.go',
            cwd: './consumer',
            interpreter: 'none'
        }
    ]
};
