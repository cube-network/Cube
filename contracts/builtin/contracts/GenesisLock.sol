// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.4;

contract GenesisLock {
    uint256 public startTime;
    uint256 public periodTime;// = 2592000; //60*60*24*30 = 2592000

    /**
    * userType:
    * 1 community pool,
    * 2 private sale,
    * 3 team,
    * 4 Ecosystem foundation
    */
    mapping(address => uint256) public userType; 
    //user's lock-up amount
    mapping(address => uint256) public userLockedAmount;
    //time of user's first locked time length
    mapping(address => uint256) public firstPeriodLockedTime;
    //user's total period amount of locked Asset
    mapping(address => uint256) public lockedPeriodAmount;
    //user's current time after claim
    mapping(address => uint256) public currentTimestamp;
    //user's claimed Period
    mapping(address => uint256) public claimedPeriod;

    function initialize(uint256 _periodTime) external {
        // #if Mainnet
        require(block.number == 0,"need gensis block");
        // #endif
        require(periodTime == 0, "already initialized");
        require(_periodTime > 0, "invalid periodTime");
        periodTime = _periodTime;
    }
    /**
    *   init the data of all users
    *   The input parameters are 5 equal-length arrays, which store the user's account address, userType, userLockedAmount, firstPeriodLockedTime, and lockedPeriodAmount.
    *   The elements of the above 5 arrays need to be strictly indexed to prevent data errors
    */
    function init(
        address[] memory userAddress, 
        uint256[] memory typeId, 
        uint256[] memory lockedAmount,
        uint256[] memory lockedTime,
        uint256[] memory periodAmount
    ) external {
        // #if Mainnet
        require(block.number == 0,"need gensis block");
        // #endif
        require(periodTime > 0, "not initialized");

        require(userAddress.length == typeId.length,"typeId length must equal userAddress");
        require(userAddress.length == lockedAmount.length,"lockedAmount length must equal userAddress");
        require(userAddress.length == lockedTime.length,"lockedTime length must equal userAddress");
        require(userAddress.length == periodAmount.length,"periodAmount length must equal userAddress");
        for(uint256 i =0;i < userAddress.length;i++){
            userType[userAddress[i]] = typeId[i];
            userLockedAmount[userAddress[i]] = lockedAmount[i];
            firstPeriodLockedTime[userAddress[i]] = lockedTime[i];
            lockedPeriodAmount[userAddress[i]] = periodAmount[i];
        }
        startTime = block.timestamp;
    }

    /**
    *   user claim the unlocked asset
    */
    function claim() external {
        (uint256 claimableAmt ,uint256 period) = getClaimableAmount(msg.sender);
        require(claimableAmt > 0 && period >0,"Have no token released");

        uint256 startTimestamp = startTime + firstPeriodLockedTime[msg.sender];

        if(currentTimestamp[msg.sender] == 0){
            currentTimestamp[msg.sender] = startTimestamp + periodTime * period;
        }else{
            currentTimestamp[msg.sender] = currentTimestamp[msg.sender] + periodTime * period;
        }
        claimedPeriod[msg.sender] += period;
           
        (bool success,) = msg.sender.call{value: claimableAmt}(new bytes(0));
        require(success,"transfer failed!");
    }

    // query the Claimable Amount 
    function getClaimableAmount(address account) public view returns (uint256 claimableAmt, uint256 period) {
        period = getClaimablePeriod(account);
        claimableAmt = userLockedAmount[account] / lockedPeriodAmount[account] * period;
    }

    // query the Claimable Period
    function getClaimablePeriod(address account) public view returns (uint256 period){
        uint256 startTimestamp = startTime + firstPeriodLockedTime[account];
        uint256 maxClaimablePeriod = lockedPeriodAmount[account] - claimedPeriod[account];
        if(maxClaimablePeriod > 0){
            if(currentTimestamp[account] >= startTimestamp){
                if(block.timestamp > currentTimestamp[account]){
                    period = (block.timestamp - currentTimestamp[account]) / periodTime;
                }
            }else{
                if(block.timestamp > startTimestamp){
                    period = (block.timestamp - startTimestamp) / periodTime;
                }
            }

            if(period > maxClaimablePeriod){
                period = maxClaimablePeriod;
            }
        }
    }

    /**
    * query the released 
    */  
    function getUserReleasedPeriod(address account) internal view returns(uint256 period) {
        uint256 startTimestamp = startTime + firstPeriodLockedTime[account];
        if(block.timestamp > startTimestamp){
            period = (block.timestamp - startTimestamp) / periodTime;
            if(period > lockedPeriodAmount[account]){
                period = lockedPeriodAmount[account];
            }
        }
    }

    /**
    * query the base info
    */
    function getUserInfo(address account) external view returns(
        uint256 typId,
        uint256 lockedAount,
        uint256 firstLockTime,
        uint256 totalPeriod,
        uint256 releases
    ) {
        typId = userType[account];
        lockedAount = userLockedAmount[account];
        firstLockTime = firstPeriodLockedTime[account];
        totalPeriod = lockedPeriodAmount[account];
        releases = getUserReleasedPeriod(account);
    }

    // #if !Mainnet

    // code for testcase
    uint256 result;

    function add() external returns (uint256){
        result = result + 1;
        return result;
    }
    function getBlocktimestamp() external view returns(uint256) {
        return block.timestamp;
    }
    receive() external payable {}
    // #endif
}
