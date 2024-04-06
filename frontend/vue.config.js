// const CompressionPlugin = require('compression-webpack-plugin')

module.exports = {
    productionSourceMap: false,
    assetsDir:"static",
    // configureWebpack: config => {
    //     if (process.env.NODE_ENV === 'production') {
    //         return {
    //             plugins: [new CompressionPlugin({
    //                 test: /\.js$|\.css$/,
    //                 threshold: 4096,
    //                 deleteOriginalAssets: true,
    //                 filename: '[path][base]?gz'
    //             })]
    //         }
    //     }
    // }
}