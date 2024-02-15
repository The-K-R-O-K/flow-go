import Crypto
import "FlowToken"

access(all)
contract EVM {

    pub event BridgedAccountCreated(addressBytes: [UInt8; 20])

    /// EVMAddress is an EVM-compatible address
    access(all)
    struct EVMAddress {

        /// Bytes of the address
        access(all)
        let bytes: [UInt8; 20]

        /// Constructs a new EVM address from the given byte representation
        init(bytes: [UInt8; 20]) {
            self.bytes = bytes
        }

        /// Balance of the address
        access(all)
        fun balance(): Balance {
            let balance = InternalEVM.balance(
                address: self.bytes
            )
            return Balance(attoflow: balance)
        }
    }

    access(all)
    struct Balance {

        /// The balance in atto-FLOW
        /// Atto-FLOW is the smallest denomination of FLOW (1e18 FLOW)
        /// that is used to store account balances inside EVM 
        /// similar to the way WEI is used to store ETH divisible to 18 decimal places.
        access(all)
        var attoflow: UInt

        /// Constructs a new balance
        access(all)
        init(attoflow: UInt) {
            self.attoflow = attoflow
        }

        /// Sets the balance by a UFix64 (8 decimal points), the format 
        /// that is used in Cadence to store FLOW tokens.  
        access(all)
        fun setFLOW(flow: UFix64){
            self.attoflow = InternalEVM.castToAttoFLOW(balance: flow)
        }

        /// Casts the balance to a UFix64 (rounding down)
        /// Warning! casting a balance to a UFix64 which supports a lower level of precision 
        /// (8 decimal points in compare to 18) might result in rounding down error.
        /// Use the toAttoFlow function if you care need more accuracy. 
        access(all)
        fun inFLOW(): UFix64 {
            return InternalEVM.castToFLOW(balance: self.attoflow)
        }

        /// Returns the balance in Atto-FLOW
        access(all)
        fun inAttoFLOW(): UInt {
            return self.attoflow
        }
    }

    access(all)
    resource interface Addressable {
        /// The EVM address
        access(all)
        fun address(): EVMAddress
    }

    access(all)
    resource BridgedAccount: Addressable  {

        access(self)
        var addressBytes: [UInt8; 20]

        init() {
            // address is initially set to zero
            // but updated through initAddress later
            // we have to do this since we need resource id (uuid)
            // to calculate the EVM address for this bridge account
            self.addressBytes = [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0] 
        }

        access(contract)
        fun initAddress(addressBytes: [UInt8; 20]) {
           // only allow set address for the first time
           // check address is empty
            for item in self.addressBytes {
                assert(item == 0, message: "address byte is not empty")
            }
           self.addressBytes = addressBytes
        }

        /// The EVM address of the bridged account
        access(all)
        fun address(): EVMAddress {
            // Always create a new EVMAddress instance
            return EVMAddress(bytes: self.addressBytes)
        }

        /// Get balance of the bridged account
        access(all)
        fun balance(): Balance {
            return self.address().balance()
        }

        /// Deposits the given vault into the bridged account's balance
        access(all)
        fun deposit(from: @FlowToken.Vault) {
            InternalEVM.deposit(
                from: <-from,
                to: self.addressBytes
            )
        }

        /// Withdraws the balance from the bridged account's balance
        /// Note that amounts smaller than 10nF (10e-8) can't be withdrawn 
        /// given that Flow Token Vaults use UFix64s to store balances.
        /// If the given balance conversion to UFix64 results in 
        /// rounding error, this function would fail. 
        access(all)
        fun withdraw(balance: Balance): @FlowToken.Vault {
            let vault <- InternalEVM.withdraw(
                from: self.addressBytes,
                amount: balance.attoflow
            ) as! @FlowToken.Vault
            return <-vault
        }

        /// Deploys a contract to the EVM environment.
        /// Returns the address of the newly deployed contract
        access(all)
        fun deploy(
            code: [UInt8],
            gasLimit: UInt64,
            value: Balance
        ): EVMAddress {
            let addressBytes = InternalEVM.deploy(
                from: self.addressBytes,
                code: code,
                gasLimit: gasLimit,
                value: value.attoflow
            )
            return EVMAddress(bytes: addressBytes)
        }

        /// Calls a function with the given data.
        /// The execution is limited by the given amount of gas
        access(all)
        fun call(
            to: EVMAddress,
            data: [UInt8],
            gasLimit: UInt64,
            value: Balance
        ): [UInt8] {
             return InternalEVM.call(
                 from: self.addressBytes,
                 to: to.bytes,
                 data: data,
                 gasLimit: gasLimit,
                 value: value.attoflow
            )
        }
    }

    /// Creates a new bridged account
    access(all)
    fun createBridgedAccount(): @BridgedAccount {
        let acc <-create BridgedAccount()
        let addr = InternalEVM.createBridgedAccount(uuid: acc.uuid)
        acc.initAddress(addressBytes: addr)
        emit BridgedAccountCreated(addressBytes: addr)
        return <-acc
    }

    /// Runs an a RLP-encoded EVM transaction, deducts the gas fees,
    /// and deposits the gas fees into the provided coinbase address.
    ///
    /// Returns true if the transaction was successful,
    /// and returns false otherwise
    access(all)
    fun run(tx: [UInt8], coinbase: EVMAddress) {
        InternalEVM.run(tx: tx, coinbase: coinbase.bytes)
    }

    access(all)
    fun encodeABI(_ values: [AnyStruct]): [UInt8] {
        return InternalEVM.encodeABI(values)
    }

    access(all)
    fun decodeABI(types: [Type], data: [UInt8]): [AnyStruct] {
        return InternalEVM.decodeABI(types: types, data: data)
    }

    access(all)
    fun encodeABIWithSignature(
        _ signature: String,
        _ values: [AnyStruct]
    ): [UInt8] {
        let methodID = HashAlgorithm.KECCAK_256.hash(
            signature.utf8
        ).slice(from: 0, upTo: 4)
        let arguments = InternalEVM.encodeABI(values)

        return methodID.concat(arguments)
    }

    access(all)
    fun decodeABIWithSignature(
        _ signature: String,
        types: [Type],
        data: [UInt8]
    ): [AnyStruct] {
        let methodID = HashAlgorithm.KECCAK_256.hash(
            signature.utf8
        ).slice(from: 0, upTo: 4)

        for byte in methodID {
            if byte != data.removeFirst() {
                panic("signature mismatch")
            }
        }

        return InternalEVM.decodeABI(types: types, data: data)
    }

    /// validateCOAOwnershipProof validates a COA ownership proof
    access(all)
    fun validateCOAOwnershipProof(
        address: Address,
        path: PublicPath,
        signedData: [UInt8],
        keyIndices: [UInt64],
        signatures: [[UInt8]],
        evmAddress: [UInt8; 20]
    ) {

        // make signature set first 
        // check number of signatures matches number of key indices
        assert(keyIndices.length == signatures.length,
               message: "key indices size doesn't match the signatures")

        var signatureSet: [Crypto.KeyListSignature] = []
        var idx = 0 
        for sig in signatures{
            signatureSet.append(Crypto.KeyListSignature(
                keyIndex: Int(keyIndices[Int(idx)]),
                signature: sig
            ))
            idx = idx + 1
        }

        // fetch account
        let acc = getAccount(address)

        // constructing key list
        let keyList = Crypto.KeyList()
        for sig in signatureSet {
            let key = acc.keys.get(keyIndex: sig.keyIndex)!
            assert(!key.isRevoked, message: "revoked key is used")
            keyList.add(
              key.publicKey,
              hashAlgorithm: key.hashAlgorithm,
              weight: key.weight,
           )
        }

        let isValid = keyList.verify(
            signatureSet: signatureSet,
            signedData: signedData
        )
        assert(isValid, message: "signatures not valid")

        let coaRef = acc.getCapability(path)
            .borrow<&EVM.BridgedAccount{EVM.Addressable}>()
            ?? panic("could not borrow bridge account's address")

        // verify evm address matching
        var i = 0
        for item in coaRef.address().bytes {
            assert(item == evmAddress[i], message: "evm address mismatch")
            i = i +1
        }

    }
}
