Vue.component('signature-pad', {
  template: '#signaturepad',
  delimiters: ['${', '}'],
  data () {
    return {
      signaturePad: null,
      dataUrl: null
    }
  },
  mounted () {
    this.signaturePad = new SignaturePad(this.$refs.canv, {
      onEnd: () => {
        this.dataUrl = this.signaturePad.toDataURL()
        // as example collect all url in parent
        this.$emit('update', this.dataUrl)
      },
      backgroundColor: 'rgb(255, 255, 255)'
    })
  }
})

new Vue({
  el: '#app',
  data: {
    signaturesNeeded: 2,
    // all signature urls as example
    signatureDataUris: []
  },
  methods: {
    submitForm: function (x) {
      console.log('here')
      fetch('/echo/html', { method: 'POST',
        body: new FormData(x.target) })
        .then(() => {
          var button = document.getElementById('button')
          button.innerText = 'Sent!'
        })
    },

    updateSignature (index, url) {
      Vue.set(this.signatureDataUris, index, url)
      console.log(this.signatureDataUris)
    }
  }
})
